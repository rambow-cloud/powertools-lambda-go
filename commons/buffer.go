package commons

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// DecodeBase64Buffer follows Buffer's permissive decoding, accepting URL-safe
// symbols and ignoring non-alphabet characters. FromBase64 remains strict.
func DecodeBase64Buffer(value string) []byte {
	value, _, _ = strings.Cut(value, "=")
	value = strings.Map(func(r rune) rune {
		switch {
		case r == '-':
			return '+'
		case r == '_':
			return '/'
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/':
			return r
		default:
			return -1
		}
	}, value)
	decoded, _ := base64.RawStdEncoding.DecodeString(value)
	return decoded
}

// DecodeUTF8 replaces each maximal malformed subpart, as Buffer.toString does.
func DecodeUTF8(input []byte) string {
	var output strings.Builder
	output.Grow(len(input))
	for len(input) > 0 {
		r, size := utf8.DecodeRune(input)
		if r != utf8.RuneError || size > 1 {
			output.Write(input[:size])
			input = input[size:]
			continue
		}
		length, lower, upper := 1, byte(0x80), byte(0xbf)
		switch first := input[0]; {
		case first >= 0xc2 && first <= 0xdf:
			length = 2
		case first >= 0xe0 && first <= 0xef:
			length = 3
			if first == 0xe0 {
				lower = 0xa0
			}
			if first == 0xed {
				upper = 0x9f
			}
		case first >= 0xf0 && first <= 0xf4:
			length = 4
			if first == 0xf0 {
				lower = 0x90
			}
			if first == 0xf4 {
				upper = 0x8f
			}
		}
		consumed := 1
		for consumed < length && consumed < len(input) {
			if input[consumed] < lower || input[consumed] > upper {
				break
			}
			consumed++
			lower, upper = 0x80, 0xbf
		}
		output.WriteRune(utf8.RuneError)
		input = input[consumed:]
	}
	return output.String()
}
