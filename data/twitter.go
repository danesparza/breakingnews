package data

import (
	"context"
	"net/url"

	"github.com/ChimeraCoder/anaconda"
)

// CNNService represents the CNN news service
type CNNService struct{}

// GetNewsReport gets breaking news from CNN
func (s CNNService) GetNewsReport(ctx context.Context) (NewsReport, error) {
	retval := NewsReport{}

	api := anaconda.NewTwitterApiWithCredentials(
		"9389192-d1ou8E4ozq94uiv7NOZoboGpOHUTceLEcPz919KgbP",
		"0OnA1tVhkSvZIf1tidSJaEjmQJcNbEyBmKng45bYU1AEB",
		"zQ5sVIq5KwZxDgR7Jc7n9ILJF",
		"qcS1zokHAyeowgT2hrkzz1ljewkuw89DYB3uPuCnXt2QK1IYvp")

	//	Set some url values:
	v := url.Values{}
	v.Set("screen_name", "cnnbrk")
	v.Set("exclude_replies", "1")
	v.Set("include_rts", "false")
	v.Set("count", "50")

	//	Get the user timeline for
	timeline, _ := api.GetUserTimeline(v)

	//	Get the tweets with media (photos) and return them
	for _, tweet := range timeline {
		for _, media := range tweet.Entities.Media {

			//	If we found one with media, write out the
			//	tweet and the media and break out of the
			//	outer range loop
			tweetedTime, err := tweet.CreatedAtTime()

			if err == nil {
				retval.Items = append(retval.Items, NewsItem{
					ID:         tweet.Id,
					CreateTime: tweetedTime.Unix(),
					Text:       tweet.Text,
					MediaURL:   media.Media_url})
			}
		}
	}

	return retval, nil
}
