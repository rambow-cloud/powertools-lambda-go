package parser

import (
	"fmt"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// Zod's length check also runs on strings and JSON objects with a length property,
// even when the array type check has already failed.
func minimumLength(input any, minimum []int) []Issue {
	if len(minimum) == 0 {
		return nil
	}
	var length float64
	var label string
	switch value := input.(type) {
	case []any:
		length = float64(len(value))
		label = fmt.Sprintf("array to have >=%d items", minimum[0])
	case string:
		for _, r := range value {
			length++
			if r > 0xffff {
				length++
			}
		}
		label = fmt.Sprintf("string to have >=%d characters", minimum[0])
	default:
		object, ok := objectValue(input)
		if !ok {
			return nil
		}
		value, present := object["length"]
		if !present {
			return nil
		}
		length = commons.ParseNumber(lengthText(value))
		label = fmt.Sprintf("unknown to be >=%d", minimum[0])
	}
	if length >= float64(minimum[0]) {
		return nil
	}
	return []Issue{{Code: "too_small", Message: "Too small: expected " + label, Continuable: true}}
}

func lengthText(input any) string {
	if input == nil {
		return ""
	}
	switch value := input.(type) {
	case bool:
		if value {
			return "1"
		}
		return "0"
	case []any:
		parts := make([]string, len(value))
		for i, item := range value {
			if item == true {
				parts[i] = "true"
			} else if item == false {
				parts[i] = "false"
			} else {
				parts[i] = lengthText(item)
			}
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(input)
	}
}
