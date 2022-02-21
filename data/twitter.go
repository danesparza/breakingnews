package data

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
	log "github.com/sirupsen/logrus"
)

// TwitterTimelineResponse represents the response to a timeline request
type TwitterTimelineResponse struct {
	Tweets []Tweet `json:"data"`
	Meta   struct {
		OldestID    string `json:"oldest_id"`
		NewestID    string `json:"newest_id"`
		ResultCount int    `json:"result_count"`
		NextToken   string `json:"next_token"`
	} `json:"meta"`
}

// Tweet represents an individual tweet
type Tweet struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	Entities  struct {
		Annotations []struct {
			Start          int     `json:"start"`
			End            int     `json:"end"`
			Probability    float64 `json:"probability"`
			Type           string  `json:"type"`
			NormalizedText string  `json:"normalized_text"`
		} `json:"annotations"`
		Urls []struct {
			Start       int    `json:"start"`
			End         int    `json:"end"`
			URL         string `json:"url"`
			ExpandedURL string `json:"expanded_url"`
			DisplayURL  string `json:"display_url"`
		} `json:"urls"`
	} `json:"entities"`
	ID string `json:"id"`
}

// TwitterCNNService represents the CNN news service
type TwitterCNNService struct{}

// GetNewsReport gets breaking news from CNN
func (s TwitterCNNService) GetNewsReport(ctx context.Context) (NewsReport, error) {
	//	Start the service segment
	ctx, seg := xray.BeginSubsegment(ctx, "twittercnn-service")

	retval := NewsReport{}

	//	Get the api key:
	apikey := os.Getenv("TWITTER_V2_BEARER_TOKEN")
	if apikey == "" {
		seg.AddError(fmt.Errorf("{TWITTER_V2_BEARER_TOKEN} is blank but shouldn't be"))
		return retval, fmt.Errorf("{TWITTER_V2_BEARER_TOKEN} is blank but shouldn't be")
	}

	//	Create our request with the cnnbrk userid (you can get the userid by calling
	//	https://api.twitter.com/2/users/by/username/cnnbrk ):
	//	Fetch the url:
	clientRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.twitter.com/2/users/428333/tweets", nil)
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("cannot create request: %v", err)
	}

	//	Set our query params
	q := clientRequest.URL.Query()
	q.Add("tweet.fields", "created_at,entities")
	clientRequest.URL.RawQuery = q.Encode()

	//	Set our headers
	clientRequest.Header.Set("Content-Type", "application/json; charset=UTF-8")
	clientRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apikey))

	//	Execute the request
	client := http.Client{}
	clientResponse, err := client.Do(clientRequest)
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("error when sending request to Twitter API server: %v", err)
	}
	defer clientResponse.Body.Close()

	//	Decode the response:
	twResponse := TwitterTimelineResponse{}
	err = json.NewDecoder(clientResponse.Body).Decode(&twResponse)
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("problem decoding the response from the Twitter API server: %v", err)
	}

	//	Get the tweets with media (photos) and return them
	for _, tweet := range twResponse.Tweets {

		mediaURL := ""
		storyURL := ""
		storyDisplayURL := ""
		storyText := tweet.Text

		//	If we have an associated link, fetch it and get the image url associated (if one exists):
		if len(tweet.Entities.Urls) > 0 {
			storyURL = tweet.Entities.Urls[0].ExpandedURL
			storyDisplayURL = tweet.Entities.Urls[0].URL
			mediaURL, _ = GetTwitterImageUrlFromPage(ctx, storyURL)
		}

		//	If the story text contains the display link, remove it:
		storyText = strings.Replace(storyText, storyDisplayURL, "", 1)
		storyText = strings.TrimSpace(storyText)

		//	If we have a mediaURL
		//	...fetch the image, encode it, add it to mediadata
		//	...add the story to the collection
		if mediaURL != "" {

			mediaData, err := getResizedEncodedImage(mediaURL, 600, 300)
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"tweetID":  tweet.ID,
					"mediaUrl": mediaURL,
				}).Error("problem getting the encoded mediaData image")
			}

			retval.Items = append(retval.Items, NewsItem{
				ID:         tweet.ID,
				CreateTime: tweet.CreatedAt.Unix(),
				Text:       storyText,
				MediaURL:   mediaURL,
				MediaData:  mediaData,
				StoryURL:   storyURL})
		}

	}

	xray.AddMetadata(ctx, "TwitterCNNResult", retval)

	// Close the segment
	seg.Close(nil)

	return retval, nil
}

// GetTwitterImageUrlFromPage gets the twitter:image meta content tag contents for the given page url
// by fetching and parsing the page
func GetTwitterImageUrlFromPage(ctx context.Context, page string) (string, error) {

	//	Start the image parse segment
	ctx, seg := xray.BeginSubsegment(ctx, "twitter-image-parse")

	//	Set the initial value
	retval := ""

	//	Fetch the url:
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("cannot create request: %v", err)
	}

	client := http.Client{}
	res, err := client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("error executing request to fetch url: %v", err)
	}

	//	Read in the response
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("cannot read all of response body: %v", err)
	}

	// Find the meta item with the name 'twitter:image'
	doc.Find("meta[property='og:image']").Each(func(i int, s *goquery.Selection) {
		// For each item found, set the return value
		retval = s.AttrOr("content", "")
	})

	xray.AddMetadata(ctx, "twitter-image-result", retval)

	// Close the segment
	seg.Close(nil)

	return retval, nil
}

func getResizedEncodedImage(imageUrl string, width, height int) (string, error) {

	//	Our return value
	retval := ""

	type SubImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	//	Open the url and fetch into memory
	response, err := http.Get(imageUrl)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"url": imageUrl,
		}).Error("error fetching image url")
		return retval, fmt.Errorf("error fetching url: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return retval, fmt.Errorf("expected http 200 status code but got %v instead", response.StatusCode)
	}

	//	Analyze the image and crop it
	img, _, err := image.Decode(response.Body)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"url": imageUrl,
		}).Error("error reading source image")
		return retval, fmt.Errorf("error reading source image: %v", err)
	}

	resizer := nfnt.NewDefaultResizer()
	analyzer := smartcrop.NewAnalyzer(resizer)
	topCrop, err := analyzer.FindBestCrop(img, width, height)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"url": imageUrl,
		}).Error("error finding best crop")
		return retval, fmt.Errorf("error finding best crop: %v", err)
	}

	img = img.(SubImager).SubImage(topCrop)
	if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
		img = resizer.Resize(img, uint(width), uint(height))
	}

	//	Encode as jpg image data
	buffer := new(bytes.Buffer)
	jpeg.Encode(buffer, img, nil)

	//	base64 encode the image data and set the return value
	retval = fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(buffer.Bytes()))

	return retval, nil
}
