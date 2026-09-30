// Package commons provides shared Powertools contracts without AWS SDK dependencies.
package commons

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const ReferenceVersion = "2.35.0"
const Version = "0.1.0-dev"

type EnvironmentError struct{ Key, Expected string }

func (e *EnvironmentError) Error() string {
	if e.Expected == "" {
		return fmt.Sprintf("Environment variable %s is required", e.Key)
	}
	return fmt.Sprintf("Environment variable %s must be %s", e.Key, e.Expected)
}

// TrimSpace follows JavaScript String.trim: include BOM but retain NEXT LINE.
func TrimSpace(value string) string {
	return strings.TrimFunc(value, func(r rune) bool { return r != '\u0085' && unicode.IsSpace(r) || r == '\ufeff' })
}

func StringEnv(key string, fallback ...string) (string, error) {
	value, present := os.LookupEnv(key)
	if !present {
		if len(fallback) > 0 {
			return fallback[0], nil
		}
		return "", &EnvironmentError{Key: key}
	}
	return TrimSpace(value), nil
}

var decimalNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// ParseNumber implements the string conversions used by the pinned Number helpers.
// Invalid strings return NaN; NumberEnv turns that into a typed configuration error.
func ParseNumber(value string) float64 {
	value = TrimSpace(value)
	if value == "" {
		return 0
	}
	switch value {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(value) > 2 && value[0] == '0' {
		base := 0
		switch value[1] {
		case 'x', 'X':
			base = 16
		case 'b', 'B':
			base = 2
		case 'o', 'O':
			base = 8
		}
		if base != 0 {
			for _, c := range value[2:] {
				if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
					return math.NaN()
				}
			}
			n, ok := new(big.Int).SetString(value[2:], base)
			if !ok {
				return math.NaN()
			}
			f, _ := new(big.Float).SetInt(n).Float64()
			return f
		}
	}
	if !decimalNumber.MatchString(value) {
		return math.NaN()
	}
	n, _ := strconv.ParseFloat(value, 64)
	return n
}

func NumberEnv(key string, fallback ...float64) (float64, error) {
	value, present := os.LookupEnv(key)
	if !present && len(fallback) > 0 {
		return fallback[0], nil
	}
	if !present {
		return 0, &EnvironmentError{Key: key, Expected: "a number"}
	}
	n := ParseNumber(value)
	if math.IsNaN(n) {
		return 0, &EnvironmentError{Key: key, Expected: "a number"}
	}
	return n, nil
}

func BoolEnv(key string, extended bool, fallback ...bool) (bool, error) {
	value, present := os.LookupEnv(key)
	if !present && len(fallback) > 0 {
		return fallback[0], nil
	}
	value = strings.ToLower(TrimSpace(value))
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}
	if extended {
		switch value {
		case "1", "y", "yes", "t", "on":
			return true, nil
		case "0", "n", "no", "f", "off":
			return false, nil
		}
	}
	return false, &EnvironmentError{Key: key, Expected: "a boolean"}
}

// BoolEnvOr preserves the fallback policy of utilities whose constructors cannot return errors.
func BoolEnvOr(key string, extended, fallback bool) bool {
	value, err := BoolEnv(key, extended, fallback)
	if err != nil {
		return fallback
	}
	return value
}

func IsDevMode() bool { return BoolEnvOr("POWERTOOLS_DEV", true, false) }
func IsRunningInLambda() bool {
	kind, _ := StringEnv("AWS_LAMBDA_INITIALIZATION_TYPE", "unknown")
	return !IsDevMode() && kind != "unknown"
}
func ServiceName() string { value, _ := StringEnv("POWERTOOLS_SERVICE_NAME", ""); return value }

// ResolveServiceName keeps fallback policy explicit at the consuming utility.
func ResolveServiceName(explicit, fallback string) string {
	if TrimSpace(explicit) != "" {
		return explicit
	}
	if value := ServiceName(); value != "" {
		return value
	}
	return fallback
}
