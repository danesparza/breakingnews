package data

import (
	"context"
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/dghubble/go-twitter/twitter"
	"golang.org/x/oauth2/clientcredentials"
)

// TwitterCNNService represents the CNN news service
type TwitterCNNService struct{}

// GetNewsReport gets breaking news from CNN
func (s TwitterCNNService) GetNewsReport(ctx context.Context) (NewsReport, error) {
	//	Start the service segment
	ctx, seg := xray.BeginSubsegment(ctx, "twittercnn-service")

	retval := NewsReport{}

	// oauth2 configures a client that uses app credentials to keep a fresh token
	config := &clientcredentials.Config{
		ClientID:     "zQ5sVIq5KwZxDgR7Jc7n9ILJF",
		ClientSecret: "qcS1zokHAyeowgT2hrkzz1ljewkuw89DYB3uPuCnXt2QK1IYvp",
		TokenURL:     "https://api.twitter.com/oauth2/token",
	}
	// http.Client will automatically authorize Requests
	httpClient := config.Client(ctx)

	// Twitter client
	client := twitter.NewClient(httpClient)

	tweets, _, err := client.Timelines.UserTimeline(&twitter.UserTimelineParams{
		ScreenName: "cnnbrk",
		Count:      25,
	})
	if err != nil {
		seg.AddError(err)
		return retval, fmt.Errorf("problem getting tweets for cnnbrk: %v", err)
	}

	//	Get the tweets with media (photos) and return them
	for _, tweet := range tweets {

		//	If we found one with media, write out the
		//	tweet and the media and break out of the
		//	outer range loop
		tweetedTime, err := tweet.CreatedAtTime()

		mediaURL := ""

		//	If we have an associated url, fetch it and get the image url associated (if one exists):
		if len(tweet.Entities.Urls) > 0 {
			mediaURL, _ = GetTwitterImageUrlFromPage(ctx, tweet.Entities.Urls[0].ExpandedURL)
		}

		//	If we didn't have an error
		if err == nil && mediaURL != "" {
			retval.Items = append(retval.Items, NewsItem{
				ID:         tweet.ID,
				CreateTime: tweetedTime.Unix(),
				Text:       tweet.Text,
				MediaURL:   mediaURL})
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
	doc.Find("meta[name='twitter:image']").Each(func(i int, s *goquery.Selection) {
		// For each item found, set the return value
		retval = s.AttrOr("content", "")
	})

	xray.AddMetadata(ctx, "twitter-image-result", retval)

	// Close the segment
	seg.Close(nil)

	return retval, nil
}
