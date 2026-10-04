package parser_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func TestBase64UTF8Reference(t *testing.T) {
	raw, err := os.ReadFile("testdata/utf8-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, Kind, Input string
			Expected          any
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Cases {
		t.Run(item.Kind+"/"+item.Name, func(t *testing.T) {
			got, err := parser.Parse(context.Background(), item.Input, parser.Base64Encoded(parser.Unknown()))
			if err != nil || !reflect.DeepEqual(got, item.Expected) {
				t.Fatalf("got %#v (%v), want %#v", got, err, item.Expected)
			}
		})
	}
}
