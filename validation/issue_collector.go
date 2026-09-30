package validation

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

type issueCollector struct {
	metadata *schemaMetadata
	order    objectOrder
}

type issueGroup struct {
	keyword, location string
	instance          []string
	order             []int
	failures          []*jsonschema.ValidationError
	direct            bool
}

// Reconstruct evaluation scopes before ordering diagnostics. Sorting flattened
// issues mixes branches, array elements and property-name failures together.
func (c *issueCollector) collect(location string, instance []string, failures []*jsonschema.ValidationError) []Issue {
	return c.collectScope(location, instance, failures, "#")
}

func (c *issueCollector) collectScope(location string, instance []string, failures []*jsonschema.ValidationError, path string) []Issue {
	m := c.metadata
	var groups []*issueGroup
	children := map[string]*issueGroup{}
	var add func(*jsonschema.ValidationError)
	add = func(f *jsonschema.ValidationError) {
		switch f.ErrorKind.(type) {
		case *kind.Schema, *kind.Group, *kind.AllOf:
			for _, child := range f.Causes {
				add(child)
			}
			return
		}
		at := m.location(f.SchemaURL)
		if _, ok := f.ErrorKind.(*kind.PropertyNames); ok {
			at = strings.TrimSuffix(at, "/propertyNames")
		}
		g := &issueGroup{location: at, instance: instance, failures: []*jsonschema.ValidationError{f}, direct: true}
		if strings.HasPrefix(at, location+"/") {
			parts := strings.Split(strings.TrimPrefix(at, location+"/"), "/")
			g.keyword = parts[0]
			g.location = location + "/" + g.keyword
			g.direct = false
			switch g.keyword {
			case "properties", "patternProperties", "dependencies", "allOf", "anyOf", "oneOf":
				if len(parts) > 1 {
					g.location += "/" + parts[1]
				}
			case "items":
				if _, tuple := m.sources[location]["items"].([]any); tuple && len(parts) > 1 {
					g.location += "/" + parts[1]
				}
			}
			switch g.keyword {
			case "properties", "patternProperties", "additionalProperties", "items", "additionalItems", "contains":
				if len(f.InstanceLocation) > len(instance) {
					g.instance = f.InstanceLocation[:len(instance)+1]
				}
			}
			switch g.keyword {
			case "allOf", "anyOf", "oneOf":
				if len(parts) > 1 {
					index, _ := strconv.Atoi(parts[1])
					g.order = []int{index}
				}
			case "properties", "patternProperties", "dependencies":
				if len(parts) > 1 {
					name := strings.ReplaceAll(strings.ReplaceAll(parts[1], "~1", "/"), "~0", "~")
					g.order = []int{m.order.index(location+"/"+g.keyword, name)}
					if g.keyword == "dependencies" {
						g.order = append([]int{1}, g.order...)
					}
				}
			}
			if len(g.instance) > len(instance) {
				name := g.instance[len(instance)]
				index := c.order.index(pointer(instance), name)
				if g.keyword == "items" || g.keyword == "additionalItems" || g.keyword == "contains" {
					index, _ = strconv.Atoi(name)
				}
				g.order = append(g.order, index)
			}
			key := g.location + "\x00" + pointer(g.instance)
			if existing := children[key]; existing != nil {
				existing.failures = append(existing.failures, f)
				return
			}
			children[key] = g
		} else {
			parts := f.ErrorKind.KeywordPath()
			if len(parts) > 0 {
				g.keyword = parts[0]
			}
			switch k := f.ErrorKind.(type) {
			case *keywordError:
				if k.keyword == "dependencies" {
					name, _ := k.params["property"].(string)
					g.order = []int{0, m.order.index(location+"/dependencies", name)}
				}
			case *kind.Not:
				g.keyword = "not"
			case *kind.Dependency:
				g.keyword = "dependencies"
				g.order = []int{0, m.order.index(location+"/dependencies", k.Prop)}
			case *kind.PropertyNames:
				g.order = []int{c.order.index(pointer(instance), k.Property)}
			}
		}
		g.order = append([]int{m.rank(location, g.keyword)}, g.order...)
		groups = append(groups, g)
	}
	for _, failure := range failures {
		add(failure)
	}
	sort.SliceStable(groups, func(i, j int) bool { return slices.Compare(groups[i].order, groups[j].order) < 0 })
	issues := make([]Issue, 0)
	for _, g := range groups {
		if !g.direct {
			childPath := path + strings.TrimPrefix(schemaPath(g.location), schemaPath(location))
			issues = append(issues, c.collectScope(g.location, g.instance, g.failures, childPath)...)
			if g.keyword == "then" || g.keyword == "else" {
				issues = append(issues, Issue{InstancePath: pointer(instance), SchemaPath: path + "/if", Keyword: "if", Params: map[string]any{"failingKeyword": g.keyword}, Message: "must match \"" + g.keyword + "\" schema"})
			}
			continue
		}
		f := g.failures[0]
		switch k := f.ErrorKind.(type) {
		case *kind.Reference:
			target, causes, referencePath := c.referenceScope(location, k.URL, f.Causes)
			issues = append(issues, c.collectScope(target, f.InstanceLocation, causes, referencePath)...)
			continue
		case *kind.PropertyNames:
			child := c.collectScope(m.location(f.SchemaURL), nil, f.Causes, path+"/propertyNames")
			for i := range child {
				child[i].InstancePath = pointer(f.InstanceLocation) + child[i].InstancePath
				child[i].PropertyName = &k.Property
			}
			issues = append(issues, child...)
		case *kind.AnyOf, *kind.OneOf, *kind.Contains:
			issues = append(issues, c.collectScope(location, instance, f.Causes, path)...)
		}
		start := len(issues)
		m.collectIssues(f, &issues)
		for i := start; i < len(issues); i++ {
			if suffix, ok := strings.CutPrefix(issues[i].SchemaPath, schemaPath(location)+"/"); ok {
				issues[i].SchemaPath = path + "/" + suffix
			}
		}
		if _, ok := f.ErrorKind.(*kind.AdditionalProperties); ok {
			sort.SliceStable(issues[start:], func(i, j int) bool {
				return c.order.index(pointer(instance), issues[start+i].Params["additionalProperty"].(string)) < c.order.index(pointer(instance), issues[start+j].Params["additionalProperty"].(string))
			})
		}
	}
	return issues
}
