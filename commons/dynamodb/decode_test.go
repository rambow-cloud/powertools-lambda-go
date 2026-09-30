package dynamodb_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	ddb "github.com/rambow-cloud/powertools-lambda-go/commons/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/internal/testfixture"
)

func TestDynamoDBReferenceAndNativePrecision(t *testing.T) {
	f := testfixture.Load(t)
	for _, item := range f["numbers"].([]any) {
		test := item.(map[string]any)
		value, err := ddb.Number(test["input"].(string))
		testfixture.AssertOutcome(t, test, value, err)
	}
	value, err := ddb.UnmarshallDynamoDB(f["rawItem"].(map[string]any))
	if err != nil || !reflect.DeepEqual(testfixture.Normalize(value), f["unmarshalled"]) {
		t.Fatalf("raw attributes: %v %v", value, err)
	}
	native, err := ddb.UnmarshalItem(map[string]types.AttributeValue{
		"large":   &types.AttributeValueMemberN{Value: "9007199254740993"},
		"numbers": &types.AttributeValueMemberNS{Value: []string{"1", "9007199254740993"}},
		"bytes":   &types.AttributeValueMemberB{Value: []byte{0, 255}},
	})
	if err != nil || native["large"].(*big.Int).String() != "9007199254740993" || !reflect.DeepEqual(native["bytes"], []byte{0, 255}) {
		t.Fatalf("SDK precision: %v %v", native, err)
	}
	copy := commons.CloneValue(native).(map[string]any)
	copy["large"].(*big.Int).SetInt64(1)
	if native["large"].(*big.Int).String() != "9007199254740993" {
		t.Fatal("large integer snapshot aliased")
	}
	if native["numbers"].([]any)[1].(*big.Int).String() != "9007199254740993" {
		t.Fatal("number set lost precision")
	}
}
