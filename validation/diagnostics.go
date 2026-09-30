package validation

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func pointer(parts []string) string {
	var path strings.Builder
	for _, part := range parts {
		path.WriteByte('/')
		path.WriteString(strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1"))
	}
	return path.String()
}

func (m *schemaMetadata) collectIssues(failure *jsonschema.ValidationError, issues *[]Issue) {
	switch failure.ErrorKind.(type) {
	case *kind.Schema, *kind.Group, *kind.Reference, *kind.AllOf:
		return
	}
	location := m.location(failure.SchemaURL)
	path := schemaPath(location) + pointer(failure.ErrorKind.KeywordPath())
	item := Issue{InstancePath: pointer(failure.InstanceLocation), SchemaPath: path, Params: map[string]any{}}
	parts := failure.ErrorKind.KeywordPath()
	if len(parts) > 0 {
		item.Keyword = parts[0]
	}
	item.Message = failure.ErrorKind.LocalizedString(message.NewPrinter(language.English))
	switch k := failure.ErrorKind.(type) {
	case *keywordError:
		item.Params = commons.CloneValue(k.params).(map[string]any)
	case *kind.Dependency:
		dependencies, _ := m.sources[location]["dependencies"].(map[string]any)
		values, _ := dependencies[k.Prop].([]any)
		names := make([]string, len(values))
		for i, name := range values {
			names[i], _ = name.(string)
		}
		deps := strings.Join(names, ", ")
		word := "properties"
		if len(names) == 1 {
			word = "property"
		}
		for _, missing := range k.Missing {
			copy := item
			copy.Keyword = "dependencies"
			copy.SchemaPath = schemaPath(location) + "/dependencies"
			copy.Params = map[string]any{"property": k.Prop, "missingProperty": missing, "depsCount": len(names), "deps": deps}
			copy.Message = "must have " + word + " " + deps + " when property " + k.Prop + " is present"
			*issues = append(*issues, copy)
		}
		return
	case *kind.PropertyNames:
		item.SchemaPath = schemaPath(location)
		item.Params["propertyName"] = k.Property
		item.Message = "property name must be valid"
	case *kind.AdditionalItems:
		tuple, _ := m.sources[location]["items"].([]any)
		lengthIssue(&item, len(tuple), "more", "items")
	case *kind.Type:
		var want any = k.Want
		if len(k.Want) == 1 {
			want = k.Want[0]
		}
		if original, ok := m.types[location]; ok {
			want = original
		}
		item.Params["type"] = commons.CloneValue(want)
		if text, ok := want.(string); ok {
			item.Message = "must be " + text
		} else {
			parts := []string{}
			if types, ok := want.([]any); ok {
				for _, value := range types {
					parts = append(parts, fmt.Sprint(value))
				}
			} else {
				parts = k.Want
			}
			item.Message = "must be " + strings.Join(parts, ",")
		}
	case *kind.Required:
		for _, name := range k.Missing {
			copy := item
			copy.Params = map[string]any{"missingProperty": name}
			copy.Message = "must have required property '" + name + "'"
			*issues = append(*issues, copy)
		}
		return
	case *kind.AdditionalProperties:
		for _, name := range k.Properties {
			copy := item
			copy.Params = map[string]any{"additionalProperty": name}
			copy.Message = "must NOT have additional properties"
			*issues = append(*issues, copy)
		}
		return
	case *kind.Enum:
		item.Params["allowedValues"] = commons.CloneValue(k.Want)
		item.Message = "must be equal to one of the allowed values"
	case *kind.Const:
		item.Params["allowedValue"] = commons.CloneValue(k.Want)
		item.Message = "must be equal to constant"
	case *kind.Format:
		name := strings.TrimPrefix(k.Want, formatPrefix)
		item.Params["format"] = name
		item.Message = "must match format \"" + name + "\""
	case *kind.Pattern:
		item.Params["pattern"] = k.Want
		item.Message = "must match pattern \"" + k.Want + "\""
	case *kind.MinLength:
		lengthIssue(&item, k.Want, "fewer", "characters")
	case *kind.MaxLength:
		lengthIssue(&item, k.Want, "more", "characters")
	case *kind.MinItems:
		lengthIssue(&item, k.Want, "fewer", "items")
	case *kind.MaxItems:
		lengthIssue(&item, k.Want, "more", "items")
	case *kind.MinProperties:
		lengthIssue(&item, k.Want, "fewer", "properties")
	case *kind.MaxProperties:
		lengthIssue(&item, k.Want, "more", "properties")
	case *kind.Minimum:
		numberIssue(&item, k.Want, ">=")
	case *kind.Maximum:
		numberIssue(&item, k.Want, "<=")
	case *kind.ExclusiveMinimum:
		numberIssue(&item, k.Want, ">")
	case *kind.ExclusiveMaximum:
		numberIssue(&item, k.Want, "<")
	case *kind.MultipleOf:
		value, _ := k.Want.Float64()
		item.Params["multipleOf"] = value
		item.Message = fmt.Sprintf("must be multiple of %v", value)
	case *kind.Not:
		item.Keyword = "not"
		item.SchemaPath += "/not"
		item.Message = "must NOT be valid"
	case *kind.FalseSchema:
		item.Keyword = "false schema"
		item.SchemaPath += "/false schema"
		item.Message = "boolean schema is false"
	case *kind.AnyOf:
		item.Message = "must match a schema in anyOf"
	case *kind.OneOf:
		item.Params["passingSchemas"] = k.Subschemas
		item.Message = "must match exactly one schema in oneOf"
	case *kind.Contains:
		item.Params["minContains"] = 1
		item.Message = "must contain at least 1 valid item(s)"
	case *kind.UniqueItems:
		i, j := k.Duplicates[1], k.Duplicates[0]
		item.Params["i"] = i
		item.Params["j"] = j
		item.Message = fmt.Sprintf("must NOT have duplicate items (items ## %d and %d are identical)", j, i)
	}
	*issues = append(*issues, item)
}

func lengthIssue(issue *Issue, limit int, direction, unit string) {
	issue.Params["limit"] = limit
	issue.Message = fmt.Sprintf("must NOT have %s than %d %s", direction, limit, unit)
}
func numberIssue(issue *Issue, limit *big.Rat, comparison string) {
	value, _ := limit.Float64()
	issue.Params["comparison"] = comparison
	issue.Params["limit"] = value
	issue.Message = fmt.Sprintf("must be %s %v", comparison, value)
}
