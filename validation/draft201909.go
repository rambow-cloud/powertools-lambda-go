package validation

import (
	"context"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Draft201909Compiler selects JSON Schema Draft 2019-09 through Options.Compiler.
// Tuple items/additionalItems and recursive anchors follow this dialect's engine.
// Registration, snapshots, formats, cancellation and native diagnostics follow
// Draft202012Compiler; declarations and keywords are checked independently.
// This does not expand the default Draft 7 compiler or promise AJV error parity.
type Draft201909Compiler struct{}

const draft201909URI = "https://json-schema.org/draft/2019-09/schema"
const draft201909Keywords = " $schema $id $ref $anchor $recursiveAnchor $recursiveRef $defs $vocabulary $comment definitions dependencies type enum const multipleOf minimum maximum exclusiveMinimum exclusiveMaximum minLength maxLength pattern format minItems maxItems uniqueItems contains minContains maxContains items additionalItems minProperties maxProperties required properties patternProperties additionalProperties propertyNames dependentRequired dependentSchemas unevaluatedProperties unevaluatedItems allOf anyOf oneOf not if then else title description default examples readOnly writeOnly deprecated contentEncoding contentMediaType contentSchema "

var draft201909Structure = sync.OnceValues(func() (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2019)
	c.UseLoader(registeredOnly{})
	return c.Compile(draft201909URI)
})

func (Draft201909Compiler) Compile(ctx context.Context, input any, options CompileOptions) (Validator, error) {
	return compileNativeDraft(ctx, input, options, nativeDialect{jsonschema.Draft2019, draft201909URI, draft201909Keywords, draft201909Structure})
}
