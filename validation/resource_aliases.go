package validation

import (
	"net/url"
	"slices"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// The engine discovers nested IDs locally after loading a document. Register
// address-only proxies so references from other documents can find them too.
// Proxies point at the original node; they never copy or relocate its schema.
func (m *schemaMetadata) registerResourceAliases(compiler *jsonschema.Compiler) error {
	m.resourceAliases = map[string]bool{}
	registered := map[string]bool{}
	proxies := map[string]string{}
	for _, document := range m.documents {
		registered[document] = true
	}
	for _, document := range m.documents {
		prefix := document + "#"
		var paths []string
		for location, source := range m.sources {
			if source != nil && strings.HasPrefix(location, prefix) {
				paths = append(paths, strings.TrimPrefix(location, prefix))
			}
		}
		// A parent pointer sorts before its descendants.
		slices.Sort(paths)
		base, err := url.Parse(document)
		if err != nil {
			return err
		}
		bases := map[string]*url.URL{"": base}
		for _, path := range paths {
			parent := path
			for parent != "" {
				parent = parent[:strings.LastIndex(parent, "/")]
				if _, ok := bases[parent]; ok {
					break
				}
			}
			current := bases[parent]
			identity, _ := m.sources[prefix+path]["$id"].(string)
			if identity != "" {
				id, err := url.Parse(identity)
				if err != nil {
					return err
				}
				current = current.ResolveReference(id)
				if current.Fragment == "" && !registered[current.String()] {
					target := document + (&url.URL{Fragment: path}).String()
					// AJV replaces cross-document pointer aliases in registration order.
					// Delay registration because the engine's resources are immutable.
					proxies[current.String()] = target
				}
			}
			bases[path] = current
		}
	}
	for identity, target := range proxies {
		if err := compiler.AddResource(identity, map[string]any{"$ref": target}); err != nil {
			return err
		}
		m.resourceAliases[identity+"#"] = true
	}
	return nil
}

func (m *schemaMetadata) unwrapResourceAlias(schema *jsonschema.Schema) *jsonschema.Schema {
	seen := map[*jsonschema.Schema]bool{}
	for schema != nil && m.resourceAliases[decodedLocation(schema.Location)] && schema.Ref != nil && !seen[schema] {
		seen[schema] = true
		schema = schema.Ref
	}
	return schema
}
