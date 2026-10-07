package parser_test

import (
	"context"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

func TestTypedJSONNamesMatchCaseExactly(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	schema := parser.Typed[payload](parser.Unknown())
	for _, key := range []string{"name", "Name", "NAME"} {
		got, err := parser.Parse(context.Background(), map[string]any{key: "Ada"}, schema)
		want := ""
		if key == "name" {
			want = "Ada"
		}
		if err != nil || got.Name != want {
			t.Fatalf("%s: payload=%+v error=%v; want name=%q", key, got, err, want)
		}
	}
}
