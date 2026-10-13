package dynamodb_test

import (
	json "encoding/json/v2"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddb "github.com/rambow-cloud/powertools-lambda-go/commons/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/internal/testfixture"
)

func TestSDKNumberBoundaryReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string           `json:"version"`
		Cases   []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != 24 {
		t.Fatal("unexpected numeric corpus")
	}
	for _, item := range fixture.Cases {
		t.Run(item["input"].(string), func(t *testing.T) {
			value, err := ddb.UnmarshalAttribute(&types.AttributeValueMemberN{Value: item["input"].(string)})
			testfixture.AssertOutcome(t, item, value, err)
		})
	}
}
