package validation

import (
	"fmt"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Validate each original document, including registered but unused external
// documents. The reference's default metaschema has no installed formats.
var draft7Structure = sync.OnceValues(func() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	compiler.UseLoader(registeredOnly{})
	compiler.UseRegexpEngine(deferPattern)
	for _, name := range []string{"uri", "uri-reference"} {
		compiler.RegisterFormat(&jsonschema.Format{Name: name, Validate: func(any) error { return nil }})
	}
	return compiler.Compile("http://json-schema.org/draft-07/schema")
})

// Preserve structural constraints for the engine's Draft 7 metaschema pass.
// Compilation-only errors belong to reachable schemas, including definitions
// reached through aliases or nested resource IDs, rather than every document node.
func (m *schemaMetadata) adaptSchema(value any, location string, formats map[string]bool) {
	if schema, ok := value.(map[string]any); ok {
		// AJV selects its dialect for the registered document, not for nested
		// resources. Keep the source declaration in metadata only. The adapter
		// supplies Draft 7 conditionals over the engine's shared assertion core.
		delete(schema, "$schema")
		for _, name := range []string{"properties", "patternProperties", "definitions", "$defs", "dependencies"} {
			if children, ok := schema[name].(map[string]any); ok {
				for key, child := range children {
					if _, array := child.([]any); array && name == "dependencies" {
						continue
					}
					m.adaptSchema(child, location+pointer([]string{name, key}), formats)
				}
			}
		}
		for _, name := range []string{"allOf", "anyOf", "oneOf", "items"} {
			if children, ok := schema[name].([]any); ok {
				for i, child := range children {
					m.adaptSchema(child, fmt.Sprintf("%s/%s/%d", location, name, i), formats)
				}
			}
		}
		for _, name := range []string{"additionalProperties", "additionalItems", "contains", "propertyNames", "not", "if", "then", "else", "items"} {
			if child, present := schema[name]; present {
				if _, array := child.([]any); array && name == "items" {
					continue
				}
				m.adaptSchema(child, location+"/"+name, formats)
			}
		}
	}
	if err := adaptSchema(value, formats); err != nil {
		m.compileErrors[location] = err
	}
}

// AJV's unextended Draft 7 metaschema does not assert the regex format.
// Bind actual matchers only when walking the compiled graph, before publication.
type deferredPattern string

func deferPattern(pattern string) (jsonschema.Regexp, error) {
	return deferredPattern(pattern), nil
}

func (p deferredPattern) String() string { return string(p) }
func (p deferredPattern) MatchString(string) bool {
	panic("validation: deferred pattern used before compilation")
}

func (m *schemaMetadata) bindPatterns(s *jsonschema.Schema, options RegexOptions) error {
	if s.Pattern != nil {
		compiled, err := options.compile(s.Pattern.String())
		if err != nil {
			return err
		}
		s.Pattern = compiled
	}
	// Always-valid patterns need no matcher unless additionalProperties uses
	// their names to decide which properties are additional.
	if m.alwaysValidPatterns(s) && (s.AdditionalProperties == nil || s.AdditionalProperties == true) {
		s.PatternProperties = nil
		return nil
	}
	if len(s.PatternProperties) == 0 {
		return nil
	}
	patterns := make(map[jsonschema.Regexp]*jsonschema.Schema, len(s.PatternProperties))
	for pattern, child := range s.PatternProperties {
		compiled, err := options.compile(pattern.String())
		if err != nil {
			return err
		}
		patterns[compiled] = child
	}
	s.PatternProperties = patterns
	return nil
}
