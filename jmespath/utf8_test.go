package jmespath

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPowertoolsUTF8Reference(t *testing.T) {
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
			expression := "powertools_base64(@)"
			if item.Kind == "gzip" {
				expression = "powertools_base64_gzip(@)"
			}
			got, err := Search(expression, item.Input, WithPowertoolsFunctions())
			if err != nil || !reflect.DeepEqual(got, item.Expected) {
				t.Fatalf("got %#v (%v), want %#v", got, err, item.Expected)
			}
		})
	}
}
