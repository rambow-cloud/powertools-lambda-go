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

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type nativeDialect struct {
	draft         *jsonschema.Draft
	uri, keywords string
	structure     func() (*jsonschema.Schema, error)
}

func compileNativeDraft(ctx context.Context, input any, options CompileOptions, dialect nativeDialect) (validator Validator, err error) {
	defer recoverRegexError(&err)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := options.Regex.validate(); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(dialect.draft)
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
			// JSON v2 has no Number type. The pinned schema engine consumes
			// json.Number tokens produced by our JSON v2 exact-number decoder.
			if _, ok := value.(jsonv1.Number); ok {
				return callback(value)
			}
			return nil
		}})
	}
	structure, err := dialect.structure()
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
		return checkNativeDraft(ctx, doc, location, location+"#", resources, owners, dialect)
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
	return &nativeDraftValidator{schema: compiled}, nil
}

// Walk schema positions only; annotation and instance data are not vocabularies.
func checkNativeDraft(ctx context.Context, value any, base, position string, resources map[string]any, owners map[string]string, dialect nativeDialect) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil // The standard metaschema checks node shapes.
	}
	for _, key := range slices.Sorted(maps.Keys(object)) {
		if !strings.Contains(dialect.keywords, " "+key+" ") {
			return fmt.Errorf("unregistered keyword %q for %s", key, dialect.uri)
		}
	}
	if declaration, present := object["$schema"]; present {
		if name, ok := declaration.(string); !ok || strings.TrimSuffix(name, "#") != dialect.uri {
			return fmt.Errorf("schema dialect is not %s: %v", dialect.uri, declaration)
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
				if err := checkNativeDraft(ctx, children[key], base, position+pointer([]string{name, key}), resources, owners, dialect); err != nil {
					return err
				}
			}
		}
	}
	for _, name := range []string{"prefixItems", "items", "allOf", "anyOf", "oneOf"} {
		if children, ok := object[name].([]any); ok {
			for index, child := range children {
				if err := checkNativeDraft(ctx, child, base, fmt.Sprintf("%s/%s/%d", position, name, index), resources, owners, dialect); err != nil {
					return err
				}
			}
		}
	}
	for _, name := range []string{"items", "additionalItems", "contains", "additionalProperties", "propertyNames", "unevaluatedProperties", "unevaluatedItems", "not", "if", "then", "else", "contentSchema"} {
		if child, present := object[name]; present {
			if err := checkNativeDraft(ctx, child, base, position+"/"+name, resources, owners, dialect); err != nil {
				return err
			}
		}
	}
	return nil
}

type nativeDraftValidator struct{ schema *jsonschema.Schema }

func (v *nativeDraftValidator) Validate(ctx context.Context, input any) (issues []Issue, err error) {
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
