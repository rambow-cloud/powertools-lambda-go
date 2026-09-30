package commons

import (
	"math"
	"reflect"
	"regexp"
)

func IsRecord(value any) bool          { _, ok := object(value); return ok }
func IsString(value any) bool          { _, ok := value.(string); return ok }
func IsNull(value any) bool            { return value == nil }
func IsNullOrUndefined(value any) bool { return value == nil }
func IsRegExp(value any) bool          { _, ok := value.(*regexp.Regexp); return ok }
func IsNumber(value any) bool          { _, ok := numeric(value); return ok }
func IsIntegerNumber(value any) bool {
	n, ok := numeric(value)
	return ok && !math.IsInf(n, 0) && n == math.Trunc(n)
}
func IsStringUndefinedNullEmpty(value any) bool {
	s, ok := value.(string)
	return !ok || TrimSpace(s) == ""
}

// IsTruthy preserves Commons semantics: empty arrays/objects are false.
// It is deliberately distinct from JavaScript's language-level truthiness.
func IsTruthy(value any) bool {
	if s, ok := value.(string); ok {
		return s != ""
	}
	if n, ok := numeric(value); ok {
		return n != 0
	}
	if b, ok := value.(bool); ok {
		return b
	}
	if a, ok := array(value); ok {
		return len(a) != 0
	}
	if m, ok := object(value); ok {
		return len(m) != 0
	}
	return false
}
func GetType(value any) string {
	if value == nil {
		return "null"
	}
	if _, ok := array(value); ok {
		return "array"
	}
	if IsRecord(value) {
		return "object"
	}
	if IsString(value) {
		return "string"
	}
	if IsNumber(value) {
		return "number"
	}
	if _, ok := value.(bool); ok {
		return "boolean"
	}
	return "unknown"
}
func IsStrictEqual(left, right any) bool {
	if a, ok := numeric(left); ok {
		b, ok := numeric(right)
		return ok && a == b
	}
	return reflect.DeepEqual(left, right)
}
func numeric(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	default:
		return 0, false
	}
}
