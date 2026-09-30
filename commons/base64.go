package commons

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
)

var base64Pattern = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)

// FromBase64 preserves the reference's validation-before-encoding contract.
// Omitted encoding returns the validated input bytes; use "base64" to decode.
func FromBase64(input string, encoding ...string) ([]byte, error) {
	if len(input)%4 != 0 {
		return nil, fmt.Errorf("incorrect padding on base64 string")
	}
	if !base64Pattern.MatchString(input) {
		return nil, fmt.Errorf("invalid base64 string")
	}
	mode := "utf8"
	if len(encoding) > 0 {
		mode = strings.ToLower(encoding[0])
	}
	switch mode {
	case "", "utf8", "utf-8", "ascii", "latin1", "binary":
		return []byte(input), nil
	case "base64", "base64url":
		return base64.StdEncoding.DecodeString(input)
	case "hex":
		// Buffer hex encoding stops at the first invalid or incomplete pair.
		result := []byte{}
		for i := 0; i+1 < len(input); i += 2 {
			b, err := hex.DecodeString(input[i : i+2])
			if err != nil {
				break
			}
			result = append(result, b...)
		}
		return result, nil
	case "utf16le", "utf-16le", "ucs2", "ucs-2":
		result := make([]byte, 0, len(input)*2)
		for _, word := range utf16.Encode([]rune(input)) {
			result = append(result, byte(word), byte(word>>8))
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", mode)
	}
}
