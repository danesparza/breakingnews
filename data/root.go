package data

import (
	"context"

	"github.com/aws/aws-xray-sdk-go/xray"
)

// NewsReport defines a news report
type NewsReport struct {
	Items   NewsItems `json:"items"`
	Version string    `json:"version"`
}

// NewsItem represents a single news item
type NewsItem struct {
	ID         string `json:"id"`
	CreateTime int64  `json:"createtime"`
	Text       string `json:"text"`
	MediaURL   string `json:"mediaurl"`
	MediaData  string `json:"mediadata"`
	StoryURL   string `json:"storyurl"`
}

type NewsItems []NewsItem

//	Let NewsItems know how to sort by implementing the sort interface (https://pkg.go.dev/sort#Interface):
func (n NewsItems) Len() int {
	return len(n)
}

func (n NewsItems) Less(i, j int) bool {
	return n[i].CreateTime > n[j].CreateTime
}

func (n NewsItems) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// NewsService is the interface for all services that can fetch news data
type NewsService interface {
	// GetNewsReport gets the news report
	GetNewsReport(ctx context.Context) (NewsReport, error)
}

// GetNewsReport calls all services in parallel and returns the first result
func GetNewsReport(ctx context.Context, services []NewsService) NewsReport {

	ch := make(chan NewsReport, 1)

	//	Start the service segment
	ctx, seg := xray.BeginSubsegment(ctx, "news-report")
	defer seg.Close(nil)

	//	For each passed service ...
	for _, service := range services {

		//	Launch a goroutine for each service...
		go func(c context.Context, s NewsService) {

			//	Get its pollen report ...
			result, err := s.GetNewsReport(c)

			//	As long as we don't have an error, return what we found on the result channel
			if err == nil {
				select {
				case ch <- result:
				default:
				}
			}
		}(ctx, service)

	}

	//	Return the first result passed on the channel
	return <-ch
}
