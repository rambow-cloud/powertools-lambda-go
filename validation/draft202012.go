package validation

import (
	"context"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Draft202012Compiler selects JSON Schema Draft 2020-12 through Options.Compiler.
// It leaves the default Draft 7 compiler unchanged. Schemas and references are
// snapshotted; only registered references can load. Formats are asserted using
// the engine's built-ins and registered callbacks; content remains annotation.
// Diagnostics are native engine leaf errors, not AJV messages or evaluation order.
type Draft202012Compiler struct{}

const draft202012URI = "https://json-schema.org/draft/2020-12/schema"
const draft202012Keywords = " $schema $id $ref $anchor $dynamicAnchor $dynamicRef $defs $vocabulary $comment definitions dependencies type enum const multipleOf minimum maximum exclusiveMinimum exclusiveMaximum minLength maxLength pattern format minItems maxItems uniqueItems contains minContains maxContains prefixItems items minProperties maxProperties required properties patternProperties additionalProperties propertyNames dependentRequired dependentSchemas unevaluatedProperties unevaluatedItems allOf anyOf oneOf not if then else title description default examples readOnly writeOnly deprecated contentEncoding contentMediaType contentSchema "

var draft202012Structure = sync.OnceValues(func() (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(registeredOnly{})
	return c.Compile(draft202012URI)
})

func (Draft202012Compiler) Compile(ctx context.Context, input any, options CompileOptions) (Validator, error) {
	return compileNativeDraft(ctx, input, options, nativeDialect{jsonschema.Draft2020, draft202012URI, draft202012Keywords, draft202012Structure})
}
