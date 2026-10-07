package dynamodblocal_test

import (
	json "encoding/json/v2"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/internal/jsonvalue"
)

// sameJSON compares exact decoded number tokens and array order, while ignoring
// object member order. Invalid or ambiguous JSON is never equivalent.
func sameJSON(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a, jsonvalue.Numbers) == nil &&
		json.Unmarshal(right, &b, jsonvalue.Numbers) == nil && reflect.DeepEqual(a, b)
}

func TestJSONResponseComparison(t *testing.T) {
	for _, test := range []struct {
		name, left, right string
		equal             bool
	}{
		{"reordered exact integer", `{"ok":true,"n":9007199254740993}`, `{"n":9007199254740993,"ok":true}`, true},
		{"nested objects", `{"a":[{"x":1,"y":2}],"b":null}`, `{"b":null,"a":[{"y":2,"x":1}]}`, true},
		{"rounded integer", `{"n":9007199254740993}`, `{"n":9007199254740992}`, false},
		{"changed value", `{"ok":true}`, `{"ok":false}`, false},
		{"array order", `[1,2]`, `[2,1]`, false},
		{"duplicate member", `{"n":1,"n":1}`, `{"n":1}`, false},
		{"duplicate right member", `{"n":1}`, `{"n":1,"n":1}`, false},
		{"invalid left", `{`, `{}`, false},
		{"invalid right", `{}`, `{`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sameJSON([]byte(test.left), []byte(test.right)); got != test.equal {
				t.Fatalf("equal=%v, want %v", got, test.equal)
			}
		})
	}
}
