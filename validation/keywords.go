package validation

import (
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/message"
)

// Install rules that cannot be represented by the engine's integer limits or
// rational arithmetic. Source lookup deliberately excludes synthetic aliases:
// splitting type/enum/format must not duplicate sibling constraints.
func (m *schemaMetadata) adaptKeywords(s *jsonschema.Schema) {
	source := m.sources[decodedLocation(s.Location)]
	var sizes sizeValidator
	for _, rule := range []struct {
		name, unit string
		native     **int
	}{
		{"minLength", "characters", &s.MinLength}, {"maxLength", "characters", &s.MaxLength},
		{"minItems", "items", &s.MinItems}, {"maxItems", "items", &s.MaxItems},
		{"minProperties", "properties", &s.MinProperties}, {"maxProperties", "properties", &s.MaxProperties},
	} {
		if value, ok := source[rule.name].(json.Number); ok {
			limit, _ := value.Float64()
			*rule.native = nil
			sizes = append(sizes, sizeLimit{rule.name, rule.unit, limit})
		}
	}
	if len(sizes) > 0 {
		s.Extensions = append(s.Extensions, sizes)
	}
	if s.MultipleOf != nil {
		divisor, _ := s.MultipleOf.Float64()
		s.MultipleOf = nil
		s.Extensions = append(s.Extensions, multipleValidator(divisor))
	}
	if required, ok := source["required"].([]any); ok {
		s.Required = nil
		s.Extensions = append(s.Extensions, requiredEntries(required, len(required) < 200))
	}
	if dependencies, ok := source["dependencies"].(map[string]any); ok {
		for name, value := range dependencies {
			if values, ok := value.([]any); ok {
				delete(s.Dependencies, name)
				parts := make([]string, len(values))
				for i, value := range values {
					if value != nil {
						parts[i] = propertyText(value)
					}
				}
				s.Extensions = append(s.Extensions, dependencyValidator{name, strings.Join(parts, ", "), requiredEntries(values, true)})
			}
		}
	}
	if values, ok := source["anyOf"].([]any); ok && len(values) == 0 {
		s.Extensions = append(s.Extensions, invalidKeyword{&kind.AnyOf{}})
	}
	if values, ok := source["oneOf"].([]any); ok && len(values) == 0 {
		s.Extensions = append(s.Extensions, oneOfValidator(nil))
	}
}

type sizeLimit struct {
	keyword, unit string
	limit         float64
}
type sizeValidator []sizeLimit

func (v sizeValidator) Validate(ctx *jsonschema.ValidatorContext, value any) {
	var size int
	var unit string
	switch value := value.(type) {
	case string:
		unit = "characters"
		size = utf8.RuneCountInString(value)
	case []any:
		unit = "items"
		size = len(value)
	case map[string]any:
		unit = "properties"
		size = len(value)
	default:
		return
	}
	for _, rule := range v {
		if rule.unit != unit {
			continue
		}
		minimum := strings.HasPrefix(rule.keyword, "min")
		if minimum && float64(size) >= rule.limit || !minimum && float64(size) <= rule.limit {
			continue
		}
		direction := "more"
		if minimum {
			direction = "fewer"
		}
		ctx.AddError(&keywordError{rule.keyword, "must NOT have " + direction + " than " + numberText(rule.limit) + " " + unit, map[string]any{"limit": rule.limit}})
	}
}

type multipleValidator float64

func (v multipleValidator) Validate(ctx *jsonschema.ValidatorContext, value any) {
	number, ok := value.(json.Number)
	if !ok {
		return
	}
	input, _ := number.Float64()
	quotient := input / float64(v)
	// AJV compares the floating quotient with parseInt(quotient). Integers at
	// 1e21 and above stringify in exponential notation, so parseInt differs.
	if v != 0 && quotient == math.Trunc(quotient) && math.Abs(quotient) < 1e21 {
		return
	}
	ctx.AddError(&keywordError{"multipleOf", "must be multiple of " + numberText(float64(v)), map[string]any{"multipleOf": float64(v)}})
}

type requiredEntry struct {
	name  string
	value any
}
type requiredValidator []requiredEntry

func requiredEntries(values []any, unroll bool) requiredValidator {
	entries := make(requiredValidator, len(values))
	for i, value := range values {
		name := propertyText(value)
		// AJV code generation joins array names; loops retain the array.
		if _, array := value.([]any); array && unroll {
			value = name
		}
		entries[i] = requiredEntry{name, value}
	}
	return entries
}

func (v requiredValidator) Validate(ctx *jsonschema.ValidatorContext, value any) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, entry := range v {
		if _, present := object[entry.name]; !present {
			ctx.AddError(&keywordError{"required", "must have required property '" + entry.name + "'", map[string]any{"missingProperty": entry.value}})
		}
	}
}

type dependencyValidator struct {
	property, names string
	entries         requiredValidator
}

func (v dependencyValidator) Validate(ctx *jsonschema.ValidatorContext, value any) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	if _, present := object[v.property]; !present {
		return
	}
	word := "properties"
	if len(v.entries) == 1 {
		word = "property"
	}
	for _, entry := range v.entries {
		if _, present := object[entry.name]; !present {
			ctx.AddError(&keywordError{"dependencies", "must have " + word + " " + v.names + " when property " + v.property + " is present", map[string]any{"property": v.property, "missingProperty": entry.value, "depsCount": len(v.entries), "deps": v.names}})
		}
	}
}

type invalidKeyword struct{ kind jsonschema.ErrorKind }

func (v invalidKeyword) Validate(ctx *jsonschema.ValidatorContext, _ any) { ctx.AddError(v.kind) }

type keywordError struct {
	keyword, text string
	params        map[string]any
}

func (e *keywordError) KeywordPath() []string                   { return []string{e.keyword} }
func (e *keywordError) LocalizedString(*message.Printer) string { return e.text }

func numberText(value float64) string {
	if value == 0 {
		return "0"
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}

func propertyText(value any) string {
	switch value := value.(type) {
	case nil:
		return "null"
	case string:
		return value
	case bool:
		if value {
			return "true"
		}
		return "false"
	case json.Number:
		number, _ := value.Float64()
		return numberText(number)
	case []any:
		parts := make([]string, len(value))
		for i, item := range value {
			if item != nil {
				parts[i] = propertyText(item)
			}
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}
