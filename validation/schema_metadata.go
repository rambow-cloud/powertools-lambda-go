package validation

import (
	"fmt"
	"net/url"
	"strings"
)

// Metadata retains source locations when independent constraints are split to
// preserve AJV's allErrors behavior. The engine otherwise stops at type/enum.
type schemaMetadata struct {
	aliases         map[string]string
	types           map[string]any
	lateType        map[string]string
	sources         map[string]map[string]any
	order           objectOrder
	numberFormats   map[string]bool
	references      map[string]string
	compileErrors   map[string]error
	documents       []string
	resourceAliases map[string]bool
}

func newSchemaMetadata() *schemaMetadata {
	return &schemaMetadata{aliases: map[string]string{}, types: map[string]any{}, lateType: map[string]string{}, sources: map[string]map[string]any{}, order: objectOrder{}, numberFormats: map[string]bool{}, references: map[string]string{}, compileErrors: map[string]error{}}
}

func (m *schemaMetadata) prepare(value any, location string, source any) {
	m.documents = append(m.documents, strings.TrimSuffix(location, "#"))
	original, _ := jsonValue(source)
	for path, keys := range readObjectOrder(source) {
		m.order[location+path] = keys
	}
	m.walk(value, location, original)
}

func (m *schemaMetadata) walk(value any, location string, source any) {
	schema, ok := value.(map[string]any)
	if !ok {
		return
	}
	input, _ := source.(map[string]any)
	m.sources[location] = input
	if value, ok := input["type"]; ok {
		m.types[location] = value
		if _, array := value.([]any); array && input["nullable"] == true {
			m.types[location] = schema["type"]
		}
		name, _ := value.(string)
		if types, ok := value.([]any); ok && len(types) == 1 {
			name, _ = types[0].(string)
		}
		if name != "" && input["nullable"] != true {
			groups := map[string]string{"string": " minLength maxLength pattern format ", "number": " minimum maximum exclusiveMinimum exclusiveMaximum multipleOf format ", "array": " minItems maxItems uniqueItems items additionalItems contains ", "object": " minProperties maxProperties required properties patternProperties additionalProperties dependencies propertyNames "}
			for key := range input {
				if strings.Contains(groups[name], " "+key+" ") {
					m.lateType[location] = name
				}
			}
		}
	}
	for _, name := range []string{"properties", "patternProperties", "definitions", "$defs", "dependencies"} {
		if children, ok := schema[name].(map[string]any); ok {
			originalChildren, _ := input[name].(map[string]any)
			for key, child := range children {
				m.walk(child, location+pointer([]string{name, key}), originalChildren[key])
			}
		}
	}
	for _, name := range []string{"allOf", "anyOf", "oneOf", "items"} {
		if children, ok := schema[name].([]any); ok {
			originalChildren, _ := input[name].([]any)
			for i, child := range children {
				var original any
				if i < len(originalChildren) {
					original = originalChildren[i]
				} else if name == "allOf" && input["$ref"] != nil {
					if branch, ok := child.(map[string]any); ok && branch["$ref"] == input["$ref"] {
						m.aliases[fmt.Sprintf("%s/allOf/%d", location, i)] = location
					}
				}
				m.walk(child, fmt.Sprintf("%s/%s/%d", location, name, i), original)
			}
		}
	}
	for _, name := range []string{"additionalProperties", "additionalItems", "contains", "propertyNames", "not", "if", "then", "else", "items"} {
		if child, ok := schema[name]; ok {
			if _, array := child.([]any); !array {
				m.walk(child, location+"/"+name, input[name])
			}
		}
	}
	branches, _ := schema["allOf"].([]any)
	for _, name := range []string{"type", "const", "enum", "format"} {
		if constraint, ok := schema[name]; ok {
			branchLocation := fmt.Sprintf("%s/allOf/%d", location, len(branches))
			m.aliases[branchLocation] = location
			branches = append(branches, map[string]any{name: constraint})
			delete(schema, name)
		}
	}
	if len(branches) > 0 {
		schema["allOf"] = branches
	}
}

func decodedLocation(location string) string {
	if head, fragment, ok := strings.Cut(location, "#"); ok {
		if decoded, err := url.PathUnescape(fragment); err == nil {
			location = head + "#" + decoded
		}
	}
	return location
}

func (m *schemaMetadata) location(location string) string {
	location = decodedLocation(location)
	for {
		replacement, ok := m.aliases[location]
		if !ok {
			return location
		}
		location = replacement
	}
}

func (m *schemaMetadata) rank(location, keyword string) int {
	order := []string{"type", "$ref", "const", "enum", "not", "anyOf", "oneOf", "allOf", "if", "maximum", "minimum", "exclusiveMaximum", "exclusiveMinimum", "multipleOf", "numberFormat", "maxLength", "minLength", "pattern", "format", "maxItems", "minItems", "additionalItems", "items", "contains", "uniqueItems", "maxProperties", "minProperties", "required", "propertyNames", "additionalProperties", "dependencies", "properties", "patternProperties"}
	if keyword == "format" {
		if name, ok := m.sources[location]["format"].(string); ok && m.numberFormats[name] {
			keyword = "numberFormat"
		}
	}
	if keyword == "then" || keyword == "else" {
		keyword = "if"
	}
	if keyword == "type" && m.lateType[location] != "" {
		keyword = map[string]string{"number": "maximum", "string": "maxLength", "array": "maxItems", "object": "maxProperties"}[m.lateType[location]]
	}
	for i, key := range order {
		if key == keyword {
			return i
		}
	}
	return len(order)
}
