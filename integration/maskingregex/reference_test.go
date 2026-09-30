package maskingregex_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons/regex"
	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

func TestTypeScriptMaskingRegex(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Pattern, Flags, Format string
			LastIndex, FinalIndex  int
			Data                   json.RawMessage
			Result                 any
			Fields                 bool
		}
	}
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, item := range fixture.Cases {
		expression, err := regex.Compile(item.Pattern, item.Flags, regex.Options{})
		if err != nil {
			t.Fatal(err)
		}
		expression.SetLastIndex(item.LastIndex)
		custom, dynamic := "ignored", true
		rule := datamasking.Rule{Replace: expression.Replacer(item.Format), CustomMask: &custom, DynamicMask: &dynamic}
		options := datamasking.EraseOptions{Rule: rule}
		if item.Fields {
			options = datamasking.EraseOptions{Fields: []string{"first", "nested[*]"}, Rules: []datamasking.FieldRule{{Field: "first", Rule: rule}, {Field: "nested[*]", Rule: rule}}}
		}
		value, err := datamasking.New(datamasking.Config{}).Erase(context.Background(), item.Data, options)
		if err != nil || !reflect.DeepEqual(value, item.Result) || expression.LastIndex() != item.FinalIndex {
			t.Errorf("case %d: got %#v index %d, want %#v index %d, error %v", i, value, expression.LastIndex(), item.Result, item.FinalIndex, err)
		}
	}
}
