package logger

import (
	"bytes"
	"encoding"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/internal/jsonvalue"
)

var standardOrder = []string{"level", "message", "timestamp", "service", "cold_start", "function_arn", "function_memory_size", "function_name", "function_request_id", "sampling_rate", "xray_trace_id", "tenant_id"}

func reserved(key string) bool {
	switch key {
	case "level", "message", "timestamp", "service", "sampling_rate":
		return true
	}
	return false
}

func cloneFields(fields Fields) Fields {
	out := make(Fields, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	return out
}

// mergeFields filters reserved root fields and delegates nested/indexed merges to Commons.
func mergeFields(dst Fields, src Fields) {
	filtered := make(map[string]any, len(src))
	for k, v := range src {
		if reserved(k) {
			continue
		}
		filtered[k] = v
	}
	// Existing nested objects may belong to a parent logger; copy before merging.
	merged := commons.DeepMerge(map[string]any{}, map[string]any(dst), filtered)
	clear(dst)
	for key, value := range merged {
		dst[key] = value
	}
}

func asFields(value any) (Fields, bool) {
	switch value := value.(type) {
	case Fields:
		return value, true
	case map[string]any:
		return Fields(value), true
	default:
		return nil, false
	}
}

// A fresh normalizer shares one lazy fallback stack across the entire document.
type normalizer struct {
	replacer Replacer
	active   map[visit]bool
	stacks   bool
	source   string
	fallback *errorStack
}

func (n *normalizer) fallbackStack() errorStack {
	if n.fallback == nil {
		source := n.source
		if source == "" {
			source = "logging"
		}
		captured := captureStack(source)
		n.fallback = &captured
	}
	return *n.fallback
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

func (n *normalizer) normalize(key string, value any, depth int) (any, error) {
	if n.replacer != nil {
		value = n.replacer(key, value)
	}
	return n.normalizeValue(key, value, depth)
}

// normalizeValue follows pointers without applying the same property's replacer twice.
// Descendant properties still pass through normalize and receive the replacer.
func (n *normalizer) normalizeValue(key string, value any, depth int) (any, error) {
	if value == nil {
		return nil, nil
	}
	if depth > 100 {
		return "[Truncated]", nil
	}
	// Number implements MarshalJSONTo in Go 1.27. Its scalar representation
	// must not re-enter the custom-marshaler traversal after decoding.
	if _, ok := value.(jsonv1.Number); ok {
		return value, nil
	}
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Map || v.Kind() == reflect.Slice {
		if v.IsNil() {
			return value, nil
		}
		// The address is an opaque identity token; it is never converted to a pointer.
		ref := visit{v.Type(), v.Pointer()}
		if n.active[ref] {
			return "[Circular]", nil
		}
		n.active[ref] = true
		defer delete(n.active, ref)
	}
	if err, ok := value.(error); ok {
		var stack errorStack
		var attached bool
		if n.stacks {
			stack, attached = attachedStack(err)
			// Panic metadata is lazy too: filtering, formatters and replacers may
			// remove the error before any snapshot is needed.
			if attached && stack.source == "panic" && stack.text == "" {
				stack = n.fallbackStack()
			}
		}
		err = unwrapStack(err)
		result := Fields{"name": errorName(err), "message": err.Error(), "location": ""}
		if n.stacks {
			if !attached {
				stack = n.fallbackStack()
			}
			for key, value := range (Fields{"location": stack.location, "stack": stack.text, "stack_source": stack.source}) {
				encoded, e := n.normalize(key, value, depth+1)
				if e != nil {
					return nil, e
				}
				result[key] = encoded
			}
			if stack.truncated {
				value, e := n.normalize("stack_truncated", true, depth+1)
				if e != nil {
					return nil, e
				}
				result["stack_truncated"] = value
			}
		}
		var cause any
		if wrapped, ok := err.(interface{ Unwrap() error }); ok && wrapped.Unwrap() != nil {
			cause = wrapped.Unwrap()
		}
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			cause = joined.Unwrap()
		}
		if cause != nil {
			encoded, err := n.normalize("cause", cause, depth+1)
			if err != nil {
				return nil, err
			}
			result["cause"] = encoded
		}
		return result, nil
	}
	if _, ok := value.(json.Marshaler); ok {
		return n.normalizeJSON(key, value, depth)
	}
	if _, ok := value.(json.MarshalerTo); ok {
		return n.normalizeJSON(key, value, depth)
	}
	// Text marshalers produce a scalar; traversing their underlying containers
	// would discard the method and suppress its value or error.
	if _, ok := value.(encoding.TextMarshaler); ok {
		return value, nil
	}
	switch v.Kind() {
	case reflect.Struct:
		return n.normalizeJSON(key, value, depth)
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
			return nil, nil
		}
	case reflect.Pointer, reflect.Interface:
		return n.normalizeValue(key, v.Elem().Interface(), depth+1)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return value, nil
		}
		result := Fields{}
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			encoded, err := n.normalize(k, iter.Value().Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			result[k] = encoded
		}
		return result, nil
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return value, nil
		}
		result := make([]any, v.Len())
		for i := range result {
			encoded, err := n.normalize(fmt.Sprint(i), v.Index(i).Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = encoded
		}
		return result, nil
	}
	return value, nil
}

// Apply descendant replacers to the JSON representation of structs/marshalers,
// preserving field tags and exact numeric tokens without repeating the root callback.
func (n *normalizer) normalizeJSON(key string, value any, depth int) (any, error) {
	if n.replacer == nil {
		return value, nil
	}
	data, err := marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err = json.Unmarshal(data, &decoded, jsonvalue.Numbers); err != nil {
		return nil, err
	}
	return n.normalizeValue(key, decoded, depth+1)
}

func marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

// prepareForPrint makes a shallow copy of a map-shaped document, omitting empty
// strings and JSON null values before replacer traversal. Nested values and
// nonnil empty collections remain intact; the formatter's document is not mutated.
func prepareForPrint(value any) any {
	if _, ok := value.(json.Marshaler); ok {
		return value
	}
	if _, ok := value.(json.MarshalerTo); ok {
		return value
	}
	if _, ok := value.(encoding.TextMarshaler); ok {
		return value
	}
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Map || v.Type().Key().Kind() != reflect.String {
		return value
	}
	result := reflect.MakeMapWithSize(v.Type(), v.Len())
	iter := v.MapRange()
	for iter.Next() {
		item := iter.Value().Interface()
		if item == nil {
			continue
		}
		// A custom marshaler's JSON value or error cannot be inferred from its
		// underlying Go value. Preserve it for the normal encoding traversal.
		_, jsonCustom := item.(json.Marshaler)
		_, streamingCustom := item.(json.MarshalerTo)
		_, textCustom := item.(encoding.TextMarshaler)
		if !jsonCustom && !streamingCustom && !textCustom {
			field := reflect.ValueOf(item)
			switch field.Kind() {
			case reflect.String:
				if field.Len() == 0 {
					continue
				}
			case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
				if field.IsNil() {
					continue
				}
			}
		}
		result.SetMapIndex(iter.Key(), iter.Value())
	}
	return result.Interface()
}

func encode(fields Fields, c config) ([]byte, error) {
	n := normalizer{replacer: c.replacer, active: map[visit]bool{}, stacks: c.stackTrace, source: c.stackSource}
	return n.encode(fields, c)
}

func (n *normalizer) encode(fields Fields, c config) ([]byte, error) {
	var value any = fields
	var err error
	if c.formatter != nil {
		value, err = c.formatter(fields)
		if err != nil {
			return nil, err
		}
	}
	value, err = n.normalize("", prepareForPrint(value), 0)
	if err != nil {
		return nil, err
	}
	var data []byte
	if object, ok := asFields(value); ok {
		order := append(append([]string(nil), c.order...), standardOrder...)
		remaining := make([]string, 0, len(object))
		for k := range object {
			remaining = append(remaining, k)
		}
		sort.Strings(remaining)
		order = append(order, remaining...)
		seen := map[string]bool{}
		var out bytes.Buffer
		out.WriteByte('{')
		for _, key := range order {
			v, exists := object[key]
			if !exists || seen[key] {
				continue
			}
			if len(seen) > 0 {
				out.WriteByte(',')
			}
			seen[key] = true
			k, e := marshal(key)
			if e != nil {
				return nil, e
			}
			b, e := marshal(v)
			if e != nil {
				return nil, e
			}
			out.Write(k)
			out.WriteByte(':')
			out.Write(b)
		}
		out.WriteByte('}')
		data = out.Bytes()
	} else {
		data, err = marshal(value)
		if err != nil {
			return nil, err
		}
	}
	if c.pretty {
		pretty := jsontext.Value(data)
		if err := pretty.Indent(jsontext.WithIndent("    ")); err != nil {
			return nil, err
		}
		data = pretty
	}
	return data, nil
}
