package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type absent struct{}

func kind(value any) string {
	switch value.(type) {
	case absent:
		return "undefined"
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	r := reflect.ValueOf(value)
	switch r.Kind() {
	case reflect.Array, reflect.Slice:
		return "array"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "object"
	}
}
func invalid(expected string, input any) []Issue {
	return []Issue{{Code: "invalid_type", Expected: expected, Message: fmt.Sprintf("Invalid input: expected %s, received %s", expected, kind(input))}}
}

func String() Schema[any] {
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		if _, ok := input.(string); !ok {
			return nil, invalid("string", input), nil
		}
		return input, nil, nil
	})
}
func Boolean() Schema[any] {
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		if _, ok := input.(bool); !ok {
			return nil, invalid("boolean", input), nil
		}
		return input, nil, nil
	})
}

// Null accepts explicit null while rejecting an absent object field.
func Null() Schema[any] {
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		if input != nil {
			return nil, invalid("null", input), nil
		}
		return nil, nil, nil
	})
}
func Number() Schema[any] {
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		if kind(input) != "number" {
			return nil, invalid("number", input), nil
		}
		raw, err := json.Marshal(input)
		if err != nil {
			return nil, invalid("number", input), nil
		}
		var result float64
		if err = json.Unmarshal(raw, &result); err != nil || math.IsNaN(result) || math.IsInf(result, 0) {
			return nil, invalid("number", input), nil
		}
		return result, nil, nil
	})
}
func Unknown() Schema[any] {
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) { return commons.CloneValue(input), nil, nil })
}
func Literal(value any) Schema[any] {
	return SchemaFunc[any](func(_ context.Context, input any) (any, []Issue, error) {
		if !reflect.DeepEqual(input, value) {
			encoded, _ := json.Marshal(value)
			return nil, []Issue{{Code: "invalid_value", Message: "Invalid input: expected " + string(encoded)}}, nil
		}
		return value, nil, nil
	})
}
func Nullable[T any](schema Schema[T]) Schema[any] {
	return nullable[T]{schema}
}

// Field retains declaration order, including validation issue order. Optional
// distinguishes absence from null. WithDefault applies only to absent values.
type Field struct {
	Name         string
	Schema       Schema[any]
	Optional     bool
	defaultValue any
	hasDefault   bool
}

func (f Field) WithDefault(value any) Field {
	f.defaultValue = commons.CloneValue(value)
	f.hasDefault = true
	return f
}

type UnknownFields uint8

const (
	StripUnknown UnknownFields = iota
	PreserveUnknown
	RejectUnknown
)

type ObjectSchema struct {
	fields []Field
	policy UnknownFields
}

func Object(fields ...Field) *ObjectSchema {
	return &ObjectSchema{fields: append([]Field(nil), fields...)}
}
func (s *ObjectSchema) WithUnknownFields(policy UnknownFields) *ObjectSchema {
	copy := *s
	copy.policy = policy
	return &copy
}
func (s *ObjectSchema) Extend(fields ...Field) *ObjectSchema {
	result := &ObjectSchema{fields: append([]Field(nil), s.fields...), policy: s.policy}
	for _, field := range fields {
		found := false
		for i, current := range result.fields {
			if current.Name == field.Name {
				result.fields[i] = field
				found = true
				break
			}
		}
		if !found {
			result.fields = append(result.fields, field)
		}
	}
	return result
}

// Omit returns a schema without the named fields, retaining validation order.
func (s *ObjectSchema) Omit(names ...string) *ObjectSchema {
	removed := make(map[string]bool, len(names))
	for _, name := range names {
		removed[name] = true
	}
	result := &ObjectSchema{policy: s.policy}
	for _, field := range s.fields {
		if !removed[field.Name] {
			result.fields = append(result.fields, field)
		}
	}
	return result
}

func objectValue(input any) (map[string]any, bool) {
	if value, ok := input.(map[string]any); ok {
		return value, value != nil
	}
	if _, ok := input.(absent); ok || input == nil {
		return nil, false
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, false
	}
	var result map[string]any
	if err = json.Unmarshal(encoded, &result); err != nil {
		return nil, false
	}
	return result, result != nil
}
func (s *ObjectSchema) Validate(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s *ObjectSchema) ValidateSafe(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s *ObjectSchema) validate(ctx context.Context, input any, safe bool) (any, []Issue, error) {
	value, ok := objectValue(input)
	if !ok {
		return nil, invalid("object", input), nil
	}
	result := map[string]any{}
	known := map[string]bool{}
	var issues []Issue
	for _, field := range s.fields {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if field.Schema == nil {
			return nil, nil, fmt.Errorf("schema missing for field %s", field.Name)
		}
		if known[field.Name] {
			return nil, nil, fmt.Errorf("duplicate schema field %s", field.Name)
		}
		known[field.Name] = true
		item, present := value[field.Name]
		if !present {
			if field.hasDefault {
				result[field.Name] = commons.CloneValue(field.defaultValue)
				continue
			}
			if field.Optional {
				continue
			}
			item = absent{}
		}
		parsed, failures, err := validateSchema(ctx, item, field.Schema, safe)
		if err != nil {
			return nil, nil, err
		}
		if failures != nil {
			result[field.Name] = parsed
			if issues == nil {
				issues = []Issue{}
			}
			issues = append(issues, Prefix(failures, field.Name)...)
		} else if _, missing := parsed.(absent); missing {
			// A schema accepting absence leaves the property absent in its output.
			continue
		} else {
			result[field.Name] = parsed
		}
	}
	unknown := []string{}
	for name, item := range value {
		if !known[name] {
			unknown = append(unknown, name)
			if s.policy == PreserveUnknown {
				result[name] = commons.CloneValue(item)
			}
		}
	}
	sort.Strings(unknown)
	if s.policy == RejectUnknown && len(unknown) > 0 {
		label := "Unrecognized key: "
		if len(unknown) > 1 {
			label = "Unrecognized keys: "
		}
		quoted := make([]string, len(unknown))
		for i, key := range unknown {
			encoded, _ := json.Marshal(key)
			quoted[i] = string(encoded)
		}
		issues = append(issues, Issue{Code: "unrecognized_keys", Message: label + strings.Join(quoted, ", ")})
	}
	return result, issues, nil
}

func Array[T any](schema Schema[T], minimum ...int) Schema[[]T] {
	return arraySchema[T]{schema, append([]int(nil), minimum...)}
}

type arraySchema[T any] struct {
	schema  Schema[T]
	minimum []int
}

func (s arraySchema[T]) Validate(ctx context.Context, input any) ([]T, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s arraySchema[T]) ValidateSafe(ctx context.Context, input any) ([]T, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s arraySchema[T]) validate(ctx context.Context, input any, safe bool) ([]T, []Issue, error) {
	if s.schema == nil {
		return nil, nil, fmt.Errorf("array item schema is required")
	}
	items, ok := input.([]any)
	if !ok {
		encoded, err := json.Marshal(input)
		if err == nil {
			err = json.Unmarshal(encoded, &items)
			ok = err == nil && items != nil
		}
	}
	if !ok {
		return nil, append(invalid("array", input), minimumLength(input, s.minimum)...), nil
	}
	result := make([]T, len(items))
	var issues []Issue
	for i, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		parsed, failures, err := validateSchema(ctx, item, s.schema, safe)
		if err != nil {
			return nil, nil, err
		}
		result[i] = parsed
		if failures != nil {
			if issues == nil {
				issues = []Issue{}
			}
			issues = append(issues, Prefix(failures, i)...)
		}
	}
	return result, append(issues, minimumLength(items, s.minimum)...), nil
}

func Any[T any](schema Schema[T]) Schema[any] {
	return anySchema[T]{schema}
}
func Dictionary(schema Schema[any]) Schema[any] {
	return dictionary{schema}
}

type dictionary struct{ schema Schema[any] }

func (s dictionary) Validate(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s dictionary) ValidateSafe(ctx context.Context, input any) (any, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s dictionary) validate(ctx context.Context, input any, safe bool) (any, []Issue, error) {
	if s.schema == nil {
		return nil, nil, fmt.Errorf("dictionary value schema is required")
	}
	value, ok := objectValue(input)
	if !ok {
		return nil, invalid("record", input), nil
	}
	result := map[string]any{}
	var issues []Issue
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parsed, failures, err := validateSchema(ctx, value[key], s.schema, safe)
		if err != nil {
			return nil, nil, err
		}
		result[key] = parsed
		if failures != nil {
			if issues == nil {
				issues = []Issue{}
			}
			issues = append(issues, Prefix(failures, key)...)
		}
	}
	return result, issues, nil
}

// Transform preserves an envelope's safe-mode aggregation before converting output.
func Transform[A, B any](schema Schema[A], convert func(context.Context, A) (B, error)) Schema[B] {
	return transformed[A, B]{schema, convert}
}

type transformed[A, B any] struct {
	schema  Schema[A]
	convert func(context.Context, A) (B, error)
}

func (s transformed[A, B]) Validate(ctx context.Context, input any) (B, []Issue, error) {
	return s.validate(ctx, input, false)
}
func (s transformed[A, B]) ValidateSafe(ctx context.Context, input any) (B, []Issue, error) {
	return s.validate(ctx, input, true)
}
func (s transformed[A, B]) validate(ctx context.Context, input any, safe bool) (B, []Issue, error) {
	var zero B
	var value A
	var issues []Issue
	var err error
	if s.schema == nil || s.convert == nil {
		return zero, nil, fmt.Errorf("transform schema and callback are required")
	}
	if extended, ok := s.schema.(SafeSchema[A]); safe && ok {
		value, issues, err = extended.ValidateSafe(ctx, input)
	} else {
		value, issues, err = s.schema.Validate(ctx, input)
	}
	if issues != nil || err != nil {
		return zero, issues, err
	}
	result, err := s.convert(ctx, value)
	return result, nil, err
}
func Refine[T any](schema Schema[T], predicate func(T) bool, message string) Schema[T] {
	return refined[T]{schema, predicate, message}
}

// Typed converts validated output using ordinary Go JSON tags and types. Schema
// transforms run before decoding; incompatible output types are operational errors.
func Typed[T any](schema Schema[any]) Schema[T] {
	return Transform(schema, func(_ context.Context, value any) (T, error) {
		var result T
		data, err := json.Marshal(value)
		if err == nil {
			err = json.Unmarshal(data, &result)
		}
		return result, err
	})
}
