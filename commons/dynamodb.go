// Package commons provides shared, SDK-independent utility primitives.
package commons

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
)

type DynamoDBAttributeError struct{ Message string }

func (e *DynamoDBAttributeError) Error() string { return e.Message }

// DynamoDBNumber returns float64 for safe numbers and *big.Int for large integer strings.
// Large decimal/exponent forms fail, matching the reference BigInt conversion.
func DynamoDBNumber(value string) (any, error) {
	n := ParseNumber(value)
	if math.Abs(n) <= 9007199254740991 || math.IsInf(n, 0) || math.IsNaN(n) {
		return n, nil
	}
	text := TrimSpace(value)
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") || strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B") || strings.HasPrefix(text, "0o") || strings.HasPrefix(text, "0O") {
		base = 0
	}
	integer, ok := new(big.Int).SetString(text, base)
	if !ok {
		return nil, &DynamoDBAttributeError{Message: fmt.Sprintf("%s can't be converted to BigInt", value)}
	}
	return integer, nil
}

// UnmarshallDynamoDB maps the Commons raw AttributeValue-object API.
// Raw B values are retained unchanged, unlike SDK wire-level binary decoding.
func UnmarshallDynamoDB(input map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(input))
	for key, raw := range input {
		value, err := rawAttribute(raw)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}
func rawAttribute(raw any) (any, error) {
	attribute, ok := raw.(map[string]any)
	if !ok || len(attribute) != 1 {
		return nil, &DynamoDBAttributeError{Message: "attribute must contain exactly one type"}
	}
	for kind, value := range attribute {
		switch kind {
		case "NULL":
			return nil, nil
		case "S", "B":
			return value, nil
		case "BOOL":
			if value == nil {
				return false, nil
			}
			switch v := value.(type) {
			case bool:
				return v, nil
			case string:
				return v != "", nil
			case float64:
				return v != 0 && !math.IsNaN(v), nil
			default:
				return true, nil
			}
		case "N":
			text, ok := value.(string)
			if !ok {
				return nil, &DynamoDBAttributeError{Message: "N must be a string"}
			}
			return DynamoDBNumber(text)
		case "M":
			items, ok := value.(map[string]any)
			if !ok {
				return nil, &DynamoDBAttributeError{Message: "M must be an object"}
			}
			return UnmarshallDynamoDB(items)
		case "L", "SS", "BS", "NS":
			items, ok := value.([]any)
			if !ok {
				return nil, &DynamoDBAttributeError{Message: kind + " must be an array"}
			}
			result := []any{}
			for _, item := range items {
				converted := item
				var err error
				if kind == "L" {
					converted, err = rawAttribute(item)
				}
				if kind == "NS" {
					text, ok := item.(string)
					if !ok {
						return nil, &DynamoDBAttributeError{Message: "NS entries must be strings"}
					}
					converted, err = DynamoDBNumber(text)
				}
				if err != nil {
					return nil, err
				}
				duplicate := false
				if kind != "L" {
					for _, prior := range result {
						if reflect.DeepEqual(prior, converted) {
							duplicate = true
							break
						}
					}
				}
				if !duplicate {
					result = append(result, converted)
				}
			}
			return result, nil
		default:
			return nil, &DynamoDBAttributeError{Message: "unsupported type passed: " + kind}
		}
	}
	return nil, &DynamoDBAttributeError{Message: "empty attribute"}
}
