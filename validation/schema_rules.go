package validation

import (
	"encoding/json"
	"fmt"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const annotationKeywords = " $id $schema $comment $defs definitions title description default examples readOnly writeOnly deprecated contentEncoding contentMediaType contentSchema "

// This is AJV's compile-time optimization, not schema validation. In locations
// outside metaschema traversal, empty strings/arrays and numbers have no rules.
func alwaysValidSchema(value any) (bool, error) {
	switch value := value.(type) {
	case bool:
		return value, nil
	case json.Number:
		return true, nil
	case string:
		if value == "" {
			return true, nil
		}
	case []any:
		if len(value) == 0 {
			return true, nil
		}
	case map[string]any:
		valid := true
		for key := range value {
			if !strings.Contains(keywords, " "+key+" ") {
				return false, fmt.Errorf("strict mode: unknown keyword %q", key)
			}
			if !strings.Contains(annotationKeywords, " "+key+" ") {
				valid = false
			}
		}
		return valid, nil
	}
	return false, fmt.Errorf("invalid schema rule container")
}

func ignoredConditional(schema map[string]any) bool {
	if _, present := schema["if"]; !present {
		return false
	}
	for _, name := range []string{"then", "else"} {
		if branch, present := schema[name]; present {
			if valid, err := alwaysValidSchema(branch); err != nil || !valid {
				return false
			}
		}
	}
	return true
}

// Preserve conditional schema addresses while controlling which edges are
// compiled. The adapter installs these fields before any payload validation.
type conditionalBranches struct{ condition, then, otherwise *jsonschema.Schema }

func (*conditionalBranches) Validate(*jsonschema.ValidatorContext, any) {}

func compileConditionalBranches(ctx *jsonschema.CompilerContext, schema map[string]any) (jsonschema.SchemaExt, error) {
	if _, present := schema["if"]; !present || ignoredConditional(schema) {
		return nil, nil
	}
	branches := &conditionalBranches{condition: ctx.Enqueue([]string{"if"})}
	for _, name := range []string{"then", "else"} {
		if branch, present := schema[name]; present {
			if valid, _ := alwaysValidSchema(branch); !valid {
				compiled := ctx.Enqueue([]string{name})
				if name == "then" {
					branches.then = compiled
				} else {
					branches.otherwise = compiled
				}
			}
		}
	}
	return branches, nil
}

// Draft 7 metaschema traversal does not cover every referenced fragment. AJV
// still checks these keyword value types when compiling a reachable schema.
// Annotations have no compilation rule and must not acquire new restrictions.
func checkKeywordShapes(schema map[string]any) error {
	for name, value := range schema {
		var allowed string
		switch name {
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties":
			allowed = "number"
		case "pattern", "format", "$ref":
			allowed = "string"
		case "uniqueItems", "nullable":
			allowed = "boolean"
		case "properties", "patternProperties", "dependencies":
			allowed = "object"
		case "required", "allOf", "anyOf", "oneOf", "enum":
			allowed = "array"
		case "not", "contains", "propertyNames", "additionalProperties", "additionalItems", "if", "then", "else":
			allowed = "object boolean"
		case "items":
			allowed = "object array boolean"
		default:
			continue
		}
		actual := "null"
		switch value.(type) {
		case json.Number:
			actual = "number"
		case string:
			actual = "string"
		case bool:
			actual = "boolean"
		case []any:
			actual = "array"
		case map[string]any:
			actual = "object"
		}
		if !strings.Contains(" "+allowed+" ", " "+actual+" ") {
			return fmt.Errorf("%s must be %s", name, allowed)
		}
	}
	return nil
}
