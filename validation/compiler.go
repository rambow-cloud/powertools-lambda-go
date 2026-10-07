package validation

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const rootResource = "https://powertools.invalid/validation/root.json"
const formatPrefix = "powertools-custom-"

type jsonCompiler struct{}
type registeredOnly struct{}

func (registeredOnly) Load(location string) (any, error) {
	return nil, fmt.Errorf("schema reference is not registered: %s", location)
}

func (jsonCompiler) Compile(ctx context.Context, input any, options CompileOptions) (validator Validator, err error) {
	defer recoverRegexError(&err)
	if err = options.Regex.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	input = jsonv1.RawMessage(raw)
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(deferPattern)
	// Draft 6 supplies the shared assertions; the vocabulary below controls
	// Draft 7 conditional compilation. Original documents are validated as
	// Draft 7 before adaptation, and schema locations remain unchanged.
	compiler.DefaultDraft(jsonschema.Draft6)
	compiler.RegisterVocabulary(&jsonschema.Vocabulary{
		URL: rootResource + "/definitions",
		Subschemas: []jsonschema.SchemaPath{
			{jsonschema.Prop("$defs"), jsonschema.AllProp{}},
			{jsonschema.Prop("if")}, {jsonschema.Prop("then")}, {jsonschema.Prop("else")},
		},
		Compile: compileConditionalBranches,
	})
	compiler.UseLoader(registeredOnly{})
	metadata := newSchemaMetadata()
	formats := make(map[string]bool, len(options.Formats)+len(options.NumberFormats))
	for name, callback := range options.Formats {
		if callback == nil {
			return nil, fmt.Errorf("format %q has a nil validator", name)
		}
		formats[name] = true
		compiler.RegisterFormat(&jsonschema.Format{Name: formatPrefix + name, Validate: func(value any) error {
			if _, ok := value.(string); !ok {
				return nil
			}
			return callback(value)
		}})
	}
	for name, callback := range options.NumberFormats {
		metadata.numberFormats[name] = true
		if callback == nil {
			return nil, fmt.Errorf("format %q has a nil validator", name)
		}
		if formats[name] {
			return nil, fmt.Errorf("format %q has both string and number definitions", name)
		}
		formats[name] = true
		compiler.RegisterFormat(&jsonschema.Format{Name: formatPrefix + name, Validate: func(value any) error {
			if _, ok := value.(jsonv1.Number); ok {
				return callback(value)
			}
			return nil
		}})
	}
	register := func(location string, reference jsonv1.RawMessage) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		doc, err := metadata.prepareSchema(reference, location+"#", formats)
		if err != nil {
			return err
		}
		if err = compiler.AddResource(location, doc); err != nil {
			return err
		}
		metadata.prepare(doc, location+"#", reference)
		return nil
	}
	for _, location := range slices.Sorted(maps.Keys(options.ExternalRefs)) {
		raw, err := json.Marshal(options.ExternalRefs[location])
		if err != nil {
			return nil, err
		}
		if err := register(location, raw); err != nil {
			return nil, err
		}
	}
	base, _ := url.Parse(rootResource)
	for _, reference := range options.ExternalSchemas {
		raw, err := json.Marshal(reference)
		if err != nil {
			return nil, err
		}
		value, err := jsonValue(jsonv1.RawMessage(raw))
		if err != nil {
			return nil, err
		}
		identity := ""
		if object, ok := value.(map[string]any); ok {
			identity, _ = object["$id"].(string)
		}
		location := rootResource + "/anonymous"
		if identity != "" {
			id, err := url.Parse(strings.TrimSuffix(strings.TrimSuffix(identity, "#/"), "#"))
			if err != nil {
				return nil, err
			}
			location = base.ResolveReference(id).String()
		}
		if err := register(location, raw); err != nil {
			return nil, err
		}
	}
	doc, err := metadata.prepareSchema(input, rootResource+"#", formats)
	if err != nil {
		return nil, err
	}
	if err = compiler.AddResource(rootResource, doc); err != nil {
		return nil, err
	}
	metadata.prepare(doc, rootResource+"#", input)
	if err = metadata.registerResourceAliases(compiler); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(rootResource)
	if err != nil {
		return nil, err
	}
	if err = metadata.adaptCompiled(compiled, options.Regex); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return &jsonValidator{schema: compiled, metadata: metadata}, nil
}

type jsonValidator struct {
	schema   *jsonschema.Schema
	metadata *schemaMetadata
}

func (v *jsonValidator) Validate(ctx context.Context, input any) (result []Issue, err error) {
	defer recoverRegexError(&err)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	snapshot := jsonv1.RawMessage(raw)
	value, err := jsonValue(snapshot)
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
	collector := issueCollector{metadata: v.metadata, order: readObjectOrder(snapshot)}
	issues := collector.collect(v.metadata.location(failure.SchemaURL), nil, failure.Causes)
	return issues, nil
}

// Only schema positions are traversed; annotations and enum values are data.
func (m *schemaMetadata) prepareSchema(input any, location string, formats map[string]bool) (any, error) {
	doc, err := jsonValue(input)
	if err != nil {
		return nil, err
	}
	if schema, ok := doc.(map[string]any); ok {
		if dialect, present := schema["$schema"]; present && dialect != "" && dialect != "http://json-schema.org/schema" && dialect != "http://json-schema.org/schema#" && dialect != "http://json-schema.org/draft-07/schema#" && dialect != "http://json-schema.org/draft-07/schema" {
			return nil, fmt.Errorf("schema dialect is not registered: %v", dialect)
		}
	}
	structure, err := draft7Structure()
	if err != nil {
		return nil, err
	}
	if err = structure.Validate(doc); err != nil {
		return nil, err
	}
	m.adaptSchema(doc, location, formats)
	return doc, nil
}

const keywords = " $id $schema $ref $comment $defs definitions type enum const nullable title description default examples readOnly writeOnly deprecated format pattern minLength maxLength minimum maximum exclusiveMinimum exclusiveMaximum multipleOf properties patternProperties additionalProperties required minProperties maxProperties dependencies propertyNames items additionalItems contains minItems maxItems uniqueItems allOf anyOf oneOf not if then else contentEncoding contentMediaType contentSchema "

func adaptSchema(value any, formats map[string]bool) error {
	if _, ok := value.(bool); ok {
		return nil
	}
	schema, ok := value.(map[string]any)
	if !ok {
		if valid, _ := alwaysValidSchema(value); valid {
			return nil
		}
		return fmt.Errorf("schema must be an object or boolean")
	}
	if err := checkKeywordShapes(schema); err != nil {
		return err
	}
	if types, present := schema["type"]; present {
		if types == "" {
			delete(schema, "type")
			if _, present := schema["nullable"]; present {
				return fmt.Errorf("nullable requires type")
			}
			return adaptSchema(schema, formats)
		}
		values, array := types.([]any)
		if !array {
			values = []any{types}
		}
		for _, value := range values {
			name, ok := value.(string)
			if !ok || !strings.Contains(" array boolean integer null number object string ", " "+name+" ") {
				return fmt.Errorf("invalid schema type %v", value)
			}
		}
	}
	if _, present := schema["if"]; present {
		_, then := schema["then"]
		_, otherwise := schema["else"]
		if !then && !otherwise {
			return fmt.Errorf("if without then or else is ignored")
		}
		for _, name := range []string{"then", "else"} {
			if branch, present := schema[name]; present {
				if _, err := alwaysValidSchema(branch); err != nil {
					return err
				}
			}
		}
	} else if _, then := schema["then"]; then {
		return fmt.Errorf("then without if is ignored")
	} else if _, otherwise := schema["else"]; otherwise {
		return fmt.Errorf("else without if is ignored")
	}
	if _, present := schema["additionalItems"]; present {
		if _, tuple := schema["items"].([]any); !tuple {
			return fmt.Errorf("additionalItems requires tuple items")
		}
	}
	for name := range schema {
		if !strings.Contains(keywords, " "+name+" ") {
			return fmt.Errorf("strict mode: unknown keyword %q", name)
		}
	}
	if name, ok := schema["format"].(string); ok {
		if !formats[name] {
			return fmt.Errorf("unknown format %q", name)
		}
		schema["format"] = formatPrefix + name
	}
	if nullable, present := schema["nullable"]; present {
		enabled, ok := nullable.(bool)
		if !ok {
			return fmt.Errorf("nullable must be a boolean")
		}
		types, present := schema["type"]
		if !present {
			return fmt.Errorf("nullable requires type")
		}
		if values, ok := types.([]any); ok {
			if len(values) == 0 {
				return fmt.Errorf("type must not be empty")
			}
			if !enabled {
				for _, value := range values {
					if value == "null" {
						return fmt.Errorf("nullable false contradicts null type")
					}
				}
			}
		}
		if enabled {
			switch types := types.(type) {
			case string:
				if types != "null" {
					schema["type"] = []any{types, "null"}
				}
			case []any:
				hasNull := false
				for _, entry := range types {
					if entry == "null" {
						hasNull = true
					}
				}
				if !hasNull {
					schema["type"] = append(types, "null")
				}
			}
		} else if types == "null" {
			return fmt.Errorf("nullable false contradicts null type")
		}
		delete(schema, "nullable")
	}
	for _, name := range []string{"allOf", "anyOf", "oneOf"} {
		if raw, present := schema[name]; present {
			if _, ok := raw.([]any); !ok {
				return fmt.Errorf("%s must be an array", name)
			}
		}
	}
	if raw, present := schema["enum"]; present {
		values, ok := raw.([]any)
		if !ok || len(values) == 0 {
			return fmt.Errorf("enum must be a nonempty array")
		}
	}
	// AJV evaluates reference siblings even in draft 7. Retain their original
	// locations and move only the reference into an additional allOf branch.
	if ref, present := schema["$ref"]; present && len(schema) > 1 {
		if raw, exists := schema["allOf"]; exists {
			if _, ok := raw.([]any); !ok {
				return fmt.Errorf("allOf must be an array")
			}
		}
		branches, _ := schema["allOf"].([]any)
		schema["allOf"] = append(branches, map[string]any{"$ref": ref})
		delete(schema, "$ref")
	}
	return nil
}
