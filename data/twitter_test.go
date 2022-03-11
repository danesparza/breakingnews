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

	// t.Logf("Returned object: %+v", response)

}
