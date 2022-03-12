package data_test

import (
	"context"
	"testing"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/danesparza/breakingnews/data"
)

func TestTwitter_GetNewsReport_ReturnsValidData(t *testing.T) {
	//	Arrange
	service := data.TwitterCNNService{}
	ctx := context.Background()
	ctx, seg := xray.BeginSegment(ctx, "unit-test")
	defer seg.Close(nil)

	//	Act
	response, err := service.GetNewsReport(ctx)

	//	Assert
	if err != nil {
		t.Errorf("Error calling GetNewsReport: %v", err)
	}

	t.Logf("Returned %v items", len(response.Items))

	if len(response.Items) < 1 {
		t.Errorf("No items returned")
	}

	if len(response.Items) > 1 {
		// 	Make sure that the items are in sorted order
		previousTime := int64(0)

		for _, item := range response.Items {
			if previousTime > item.CreateTime {
				t.Errorf("Items not returned in sorted order! %+v has a create date less than %v", item, previousTime)
			}
			previousTime = item.CreateTime
		}

	}

	// t.Logf("Returned object: %+v", response)

}
