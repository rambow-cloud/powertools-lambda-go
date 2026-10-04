package parameters_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

func TestTransformUTF8Reference(t *testing.T) {
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
			var input any = item.Input
			transform, name := parameters.Binary, "fixture.binary"
			if item.Kind == "json-bytes" {
				transform, name = parameters.JSON, "fixture.json"
			}
			if item.Kind == "json-bytes" || item.Kind == "binary-bytes" {
				var err error
				input, err = base64.StdEncoding.DecodeString(item.Input)
				if err != nil {
					t.Fatal(err)
				}
				got, err := parameters.TransformValue(name, input, "")
				if err != nil || !reflect.DeepEqual(got, input) {
					t.Fatalf("raw byte input changed: %#v, %v", got, err)
				}
			}
			for _, mode := range []parameters.Transform{transform, parameters.Auto} {
				got, err := parameters.TransformValue(name, input, mode)
				if err != nil || !reflect.DeepEqual(got, item.Expected) {
					t.Errorf("%s: got %#v (%v), want %#v", mode, got, err, item.Expected)
				}
			}
		})
	}
}
