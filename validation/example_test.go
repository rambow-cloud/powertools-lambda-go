package validation_test

import (
	"context"
	"encoding/json/jsontext"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/validation"
)

func ExampleCompile() {
	ctx := context.Background()
	schema, err := validation.Compile(ctx, map[string]any{
		"type": "object", "required": []any{"orderId"},
		"properties": map[string]any{"orderId": map[string]any{"type": "string"}},
	}, validation.Options{})
	if err != nil {
		panic(err)
	}
	value, err := schema.Validate(ctx, map[string]any{"orderId": "order-123"})
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(map[string]any)["orderId"])
	// Output: order-123
}

func ExampleDraft202012Compiler() {
	schema, err := validation.Compile(context.Background(), jsontext.Value(`{
        "$schema":"https://json-schema.org/draft/2020-12/schema",
        "type":"array","prefixItems":[{"type":"integer"},{"type":"string"}],"items":false
    }`), validation.Options{Compiler: validation.Draft202012Compiler{}})
	if err != nil {
		panic(err)
	}
	_, valid := schema.Validate(context.Background(), []any{1, "ok"})
	_, extra := schema.Validate(context.Background(), []any{1, "ok", true})
	fmt.Println(valid == nil, extra != nil)
	// Output: true true
}

func ExampleDraft201909Compiler() {
	schema, err := validation.Compile(context.Background(), jsontext.Value(`{
        "$schema":"https://json-schema.org/draft/2019-09/schema",
        "type":"array","items":[{"type":"integer"},{"type":"string"}],"additionalItems":false
    }`), validation.Options{Compiler: validation.Draft201909Compiler{}})
	if err != nil {
		panic(err)
	}
	_, valid := schema.Validate(context.Background(), []any{1, "ok"})
	_, extra := schema.Validate(context.Background(), []any{1, "ok", true})
	fmt.Println(valid == nil, extra != nil)
	// Output: true true
}
