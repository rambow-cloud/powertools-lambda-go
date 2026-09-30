package regex

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// AJV's strict property-overlap check uses new RegExp(pattern) without "u".
// Preserve UTF-16 code units for both pattern atoms and names. Unicode payload
// matching continues to use the regular compiler and its pinned property data.
func legacyPattern(pattern string, dotAll bool) (string, error) {
	var result strings.Builder
	inClass := false
	for i := 0; i < len(pattern); {
		remaining := pattern[i:]
		if !inClass && strings.HasPrefix(remaining, "(?") && !strings.HasPrefix(remaining, "(?:") && !strings.HasPrefix(remaining, "(?=") && !strings.HasPrefix(remaining, "(?!") && !strings.HasPrefix(remaining, "(?<") {
			return "", fmt.Errorf("unsupported JavaScript group syntax")
		}
		if !inClass && (strings.HasPrefix(remaining, `\k<`) || (strings.HasPrefix(remaining, "(?<") && len(remaining) > 3 && remaining[3] != '=' && remaining[3] != '!')) {
			if end := strings.IndexByte(remaining, '>'); end >= 0 {
				result.WriteString(remaining[:end+1])
				i += end + 1
				continue
			}
		}
		ch, size := utf8.DecodeRuneInString(remaining)
		i += size
		if ch == '\\' && i < len(pattern) {
			escaped, size := utf8.DecodeRuneInString(pattern[i:])
			i += size
			if escaped < 128 && asciiLetter(byte(escaped)) && !strings.ContainsRune("bBfnrtvcuxdDsSwWk", escaped) {
				fmt.Fprintf(&result, `\u%04X`, escaped)
				continue
			}
			result.WriteRune(ch)
			result.WriteRune(escaped)
			continue
		}
		switch ch {
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '.':
			if !inClass {
				if dotAll {
					result.WriteString(`[\s\S]`)
				} else {
					result.WriteString(`[^\n\r\u2028\u2029]`)
				}
				continue
			}
		}
		if ch > 0xffff {
			high, low := utf16.EncodeRune(ch)
			fmt.Fprintf(&result, `\u%04X\u%04X`, high, low)
		} else {
			result.WriteRune(ch)
		}
	}
	return result.String(), nil
}
