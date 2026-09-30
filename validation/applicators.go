package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons/regex"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// Install adapters once, before the compiled graph is shared. Each adapter
// validates its keyword exactly once and keeps request state on the stack.
func (m *schemaMetadata) adaptCompiled(root *jsonschema.Schema, regex RegexOptions) error {
	seen := map[*jsonschema.Schema]bool{}
	var visit func(*jsonschema.Schema) error
	visit = func(s *jsonschema.Schema) error {
		if s == nil || seen[s] {
			return nil
		}
		seen[s] = true
		s.Ref = m.unwrapResourceAlias(s.Ref)
		if err := m.compileErrors[m.location(s.Location)]; err != nil {
			return err
		}
		if err := m.bindPatterns(s, regex); err != nil {
			return err
		}
		m.adaptKeywords(s)
		if s.Ref != nil {
			m.references[m.location(s.Location)] = m.location(s.Ref.Location)
		}
		if err := m.checkMatchingProperties(s, regex); err != nil {
			return err
		}
		retained := s.Extensions[:0]
		for _, extension := range s.Extensions {
			if branches, ok := extension.(*conditionalBranches); ok {
				s.If, s.Then, s.Else = branches.condition, branches.then, branches.otherwise
			} else {
				retained = append(retained, extension)
			}
		}
		s.Extensions = retained
		children := []*jsonschema.Schema{s.Ref, s.Not, s.If, s.Then, s.Else, s.Contains, s.PropertyNames}
		children = append(children, s.AllOf...)
		children = append(children, s.AnyOf...)
		children = append(children, s.OneOf...)
		for _, child := range s.Properties {
			children = append(children, child)
		}
		for _, child := range s.PatternProperties {
			children = append(children, child)
		}
		for _, child := range s.Dependencies {
			if schema, ok := child.(*jsonschema.Schema); ok {
				children = append(children, schema)
			}
		}
		for _, value := range []any{s.Items, s.AdditionalItems, s.AdditionalProperties} {
			switch value := value.(type) {
			case *jsonschema.Schema:
				children = append(children, value)
			case []*jsonschema.Schema:
				children = append(children, value...)
			}
		}
		for _, child := range children {
			if err := visit(child); err != nil {
				return err
			}
		}
		if len(s.OneOf) > 0 {
			s.Extensions = append(s.Extensions, oneOfValidator(s.OneOf))
			s.OneOf = nil
		}
		if s.PropertyNames != nil {
			s.Extensions = append(s.Extensions, propertyNamesValidator{s.PropertyNames})
			s.PropertyNames = nil
		}
		return nil
	}
	return visit(root)
}

type oneOfValidator []*jsonschema.Schema

func (schemas oneOfValidator) Validate(ctx *jsonschema.ValidatorContext, value any) {
	var failures []*jsonschema.ValidationError
	var passing []int
	for i, schema := range schemas {
		if err := ctx.Validate(schema, value, nil); err != nil {
			failures = append(failures, err.(*jsonschema.ValidationError))
		} else {
			passing = append(passing, i)
			if len(passing) == 2 {
				break
			}
		}
	}
	if len(passing) != 1 {
		ctx.AddErrors(failures, &kind.OneOf{Subschemas: passing})
	}
}

type propertyNamesValidator struct{ schema *jsonschema.Schema }

func (v propertyNamesValidator) Validate(ctx *jsonschema.ValidatorContext, value any) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for name := range object {
		if err := v.schema.Validate(name); err != nil {
			failure := err.(*jsonschema.ValidationError)
			failure.ErrorKind = &kind.PropertyNames{Property: name}
			failure.InstanceLocation = slices.Clone(ctx.ValueLocation())
			ctx.AddErr(failure)
		}
	}
}

// AJV's strict default rejects named properties that match patternProperties.
func (m *schemaMetadata) checkMatchingProperties(s *jsonschema.Schema, options RegexOptions) error {
	if len(s.Properties) == 0 || m.alwaysValidPatterns(s) {
		return nil
	}
	for pattern := range s.PatternProperties {
		legacy, err := regex.Compile(pattern.String(), "", options.shared())
		if err != nil {
			return err
		}
		for name := range s.Properties {
			matched, err := legacy.MatchString(name)
			if err != nil {
				return &RegexError{Pattern: pattern.String(), Err: err}
			}
			if matched {
				return fmt.Errorf("property %q matches pattern %q", name, pattern.String())
			}
		}
	}
	return nil
}

func (m *schemaMetadata) alwaysValidPatterns(s *jsonschema.Schema) bool {
	for _, child := range s.PatternProperties {
		if m.compileErrors[m.location(child.Location)] != nil {
			return false
		}
		if child.Bool != nil {
			if !*child.Bool {
				return false
			}
			continue
		}
		for key := range m.sources[m.location(child.Location)] {
			if !strings.Contains(annotationKeywords, " "+key+" ") {
				return false
			}
		}
	}
	return true
}
