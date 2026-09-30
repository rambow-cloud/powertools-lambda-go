package validation

import (
	"net/url"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// AJV's default inlines reference-free schemas and uses the literal $ref as
// their diagnostic prefix. Compiled targets start a new path at "#". Keep this
// presentation separate from canonical locations used to order engine errors.
func (c *issueCollector) referenceScope(location, target string, failures []*jsonschema.ValidationError) (string, []*jsonschema.ValidationError, string) {
	m := c.metadata
	target = m.location(target)
	ref, _ := m.sources[location]["$ref"].(string)
	parsed, _ := url.Parse(ref)
	if parsed != nil && strings.HasPrefix(parsed.Fragment, "/") {
		seen := map[string]bool{}
		for !seen[target] && onlyReference(m.sources[target]) {
			seen[target] = true
			child := soleReference(failures)
			next, ok := m.references[target]
			if child == nil || !ok {
				break
			}
			target, failures = next, child.Causes
		}
	}
	path := "#"
	if !hasReference(m.sources[target]) {
		path = ref
	}
	return target, failures, path
}

func soleReference(failures []*jsonschema.ValidationError) *jsonschema.ValidationError {
	if len(failures) != 1 {
		return nil
	}
	failure := failures[0]
	switch failure.ErrorKind.(type) {
	case *kind.Reference:
		return failure
	case *kind.Schema, *kind.Group, *kind.AllOf:
		return soleReference(failure.Causes)
	}
	return nil
}

func onlyReference(schema map[string]any) bool {
	if _, ok := schema["$ref"].(string); !ok {
		return false
	}
	for key := range schema {
		if !strings.Contains(" $ref $id $schema $comment $defs definitions title description default examples readOnly writeOnly contentEncoding contentMediaType ", " "+key+" ") {
			return false
		}
	}
	return true
}

// The reference implementation scans data-valued annotations too when deciding
// whether a schema can be inlined. This helper deliberately follows that rule.
func hasReference(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if strings.Contains(" $ref $recursiveRef $recursiveAnchor $dynamicRef $dynamicAnchor ", " "+key+" ") || hasReference(child) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if hasReference(child) {
				return true
			}
		}
	}
	return false
}
