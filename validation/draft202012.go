package validation

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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

func (Draft202012Compiler) Compile(ctx context.Context, input any, options CompileOptions) (validator Validator, err error) {
	defer recoverRegexError(&err)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := options.Regex.validate(); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(registeredOnly{})
	c.UseRegexpEngine(options.Regex.compile)
	c.AssertFormat()
	for _, name := range slices.Sorted(maps.Keys(options.Formats)) {
		callback := options.Formats[name]
		if callback == nil || name == "regex" {
			return nil, fmt.Errorf("format %q has a nil callback or overrides the reserved regex format", name)
		}
		c.RegisterFormat(&jsonschema.Format{Name: name, Validate: func(value any) error {
			if _, ok := value.(string); ok {
				return callback(value)
			}
			return nil
		}})
	}
	for _, name := range slices.Sorted(maps.Keys(options.NumberFormats)) {
		callback := options.NumberFormats[name]
		if callback == nil || name == "regex" {
			return nil, fmt.Errorf("number format %q has a nil callback or overrides the reserved regex format", name)
		}
		if _, exists := options.Formats[name]; exists {
			return nil, fmt.Errorf("format %q has both string and number definitions", name)
		}
		c.RegisterFormat(&jsonschema.Format{Name: name, Validate: func(value any) error {
			if _, ok := value.(jsonv1.Number); ok {
				return callback(value)
			}
			return nil
		}})
	}
	structure, err := draft202012Structure()
	if err != nil {
		return nil, err
	}
	resources := map[string]any{}
	owners := map[string]string{}
	register := func(location string, source any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		doc, err := jsonValue(source)
		if err != nil {
			return err
		}
		if err := structure.Validate(doc); err != nil {
			return err
		}
		uri, err := url.Parse(location)
		if err != nil || !uri.IsAbs() || uri.Fragment != "" {
			return fmt.Errorf("schema resource requires an absolute fragmentless URL: %s", location)
		}
		if _, exists := owners[location]; exists {
			return fmt.Errorf("duplicate schema resource: %s", location)
		}
		resources[location] = doc
		owners[location] = location + "#"
		return checkDraft202012(ctx, doc, location, location+"#", resources, owners)
	}
	for _, location := range slices.Sorted(maps.Keys(options.ExternalRefs)) {
		if err := register(location, options.ExternalRefs[location]); err != nil {
			return nil, err
		}
	}
	base, _ := url.Parse(rootResource)
	for _, source := range options.ExternalSchemas {
		doc, err := jsonValue(source)
		if err != nil {
			return nil, err
		}
		object, ok := doc.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("external schema requires a string $id")
		}
		identity, ok := object["$id"].(string)
		if !ok || identity == "" {
			return nil, fmt.Errorf("external schema requires a string $id")
		}
		id, err := url.Parse(identity)
		if err != nil {
			return nil, err
		}
		if err := register(base.ResolveReference(id).String(), doc); err != nil {
			return nil, err
		}
	}
	if err := register(rootResource, input); err != nil {
		return nil, err
	}
	for _, location := range slices.Sorted(maps.Keys(resources)) {
		if err := c.AddResource(location, resources[location]); err != nil {
			return nil, err
		}
	}
	compiled, err := c.Compile(rootResource)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &draft202012Validator{schema: compiled}, nil
}

// Walk schema positions only; annotation and instance data are not vocabularies.
func checkDraft202012(ctx context.Context, value any, base, position string, resources map[string]any, owners map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil // The standard metaschema checks node shapes.
	}
	for _, key := range slices.Sorted(maps.Keys(object)) {
		if !strings.Contains(draft202012Keywords, " "+key+" ") {
			return fmt.Errorf("unregistered Draft 2020-12 keyword %q", key)
		}
	}
	if dialect, present := object["$schema"]; present {
		if name, ok := dialect.(string); !ok || strings.TrimSuffix(name, "#") != draft202012URI {
			return fmt.Errorf("schema dialect is not Draft 2020-12: %v", dialect)
		}
	}
	if identity, exists := object["$id"]; exists {
		name, ok := identity.(string)
		if !ok {
			return fmt.Errorf("schema $id requires a string")
		}
		uri, err := url.Parse(name)
		if err != nil || uri.Fragment != "" {
			return fmt.Errorf("invalid schema resource $id: %v", identity)
		}
		parent, err := url.Parse(base)
		if err != nil {
			return err
		}
		base = parent.ResolveReference(uri).String()
		if previous, exists := owners[base]; exists && previous != position {
			return fmt.Errorf("duplicate schema resource: %s", base)
		}
		owners[base] = position
		// Canonicalize private IDs before indexing aliases. This preserves relative
		// reference bases when a nested resource is loaded independently.
		object["$id"] = base
		resources[base] = object
	}
	for _, name := range []string{"$defs", "definitions", "properties", "patternProperties", "dependentSchemas", "dependencies"} {
		if children, ok := object[name].(map[string]any); ok {
			for _, key := range slices.Sorted(maps.Keys(children)) {
				if name == "dependencies" {
					if _, list := children[key].([]any); list {
						continue
					}
				}
				if err := checkDraft202012(ctx, children[key], base, position+pointer([]string{name, key}), resources, owners); err != nil {
					return err
				}
			}
		}
	}
	for _, name := range []string{"prefixItems", "allOf", "anyOf", "oneOf"} {
		if children, ok := object[name].([]any); ok {
			for index, child := range children {
				if err := checkDraft202012(ctx, child, base, fmt.Sprintf("%s/%s/%d", position, name, index), resources, owners); err != nil {
					return err
				}
			}
		}
	}
	for _, name := range []string{"items", "contains", "additionalProperties", "propertyNames", "unevaluatedProperties", "unevaluatedItems", "not", "if", "then", "else", "contentSchema"} {
		if child, present := object[name]; present {
			if err := checkDraft202012(ctx, child, base, position+"/"+name, resources, owners); err != nil {
				return err
			}
		}
	}
	return nil
}

type draft202012Validator struct{ schema *jsonschema.Schema }

func (v *draft202012Validator) Validate(ctx context.Context, input any) (issues []Issue, err error) {
	defer recoverRegexError(&err)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, err := jsonValue(input)
	if err != nil {
		return nil, err
	}
	err = v.schema.Validate(value)
	if cancel := ctx.Err(); cancel != nil {
		return nil, cancel
	}
	if err == nil {
		return nil, nil
	}
	var failure *jsonschema.ValidationError
	if !errors.As(err, &failure) {
		return nil, err
	}
	printer := message.NewPrinter(language.English)
	var visit func(*jsonschema.ValidationError)
	visit = func(f *jsonschema.ValidationError) {
		if len(f.Causes) != 0 {
			for _, child := range f.Causes {
				visit(child)
			}
			return
		}
		keyword := "schema"
		path := f.ErrorKind.KeywordPath()
		if len(path) != 0 {
			keyword = path[0]
		}
		if _, ok := f.ErrorKind.(*kind.FalseSchema); ok {
			keyword = "falseSchema"
		}
		fragment := (&url.URL{Fragment: pointer(path)}).EscapedFragment()
		location := f.SchemaURL + fragment
		errorKind := f.ErrorKind
		if extra, ok := errorKind.(*kind.AdditionalProperties); ok {
			copy := *extra
			copy.Properties = slices.Clone(extra.Properties)
			slices.Sort(copy.Properties)
			errorKind = &copy
		}
		issues = append(issues, Issue{InstancePath: pointer(f.InstanceLocation), SchemaPath: location, Keyword: keyword,
			Message: errorKind.LocalizedString(printer), Params: map[string]any{"absoluteKeywordLocation": location}})
	}
	visit(failure)
	slices.SortFunc(issues, func(a, b Issue) int {
		for _, pair := range [][2]string{{a.InstancePath, b.InstancePath}, {a.SchemaPath, b.SchemaPath}, {a.Keyword, b.Keyword}, {a.Message, b.Message}} {
			if order := strings.Compare(pair[0], pair[1]); order != 0 {
				return order
			}
		}
		return 0
	})
	return issues, nil
}
