package parser_test

import (
	"context"
	json "encoding/json/v2"
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
			Name     string `json:"name"`
			Kind     string `json:"kind"`
			Input    string `json:"input"`
			Expected any    `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("reference corpus has no cases")
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
