package regex

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// unicodePattern adapts lexical differences between JavaScript Unicode patterns
// and the matching backend. It preserves group/quantifier parsing in the engine.
func unicodePattern(pattern string, dotAll bool) (string, error) {
	var result strings.Builder
	inClass := false
	classHasAtom, lastClassSet, rangePending := false, false, false
	var groups []bool
	captures, maxReference := 0, 0
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if ch == '\\' {
			i++
			if i == len(pattern) {
				return "", fmt.Errorf("trailing escape")
			}
			escape := pattern[i]
			if inClass {
				set := strings.ContainsRune("dDsSwWpP", rune(escape))
				if rangePending && set {
					return "", fmt.Errorf("character class escape cannot be a range endpoint")
				}
				classHasAtom, lastClassSet, rangePending = true, set, false
			} else if (escape == 'b' || escape == 'B') && quantifierAfter(pattern, i) {
				return "", fmt.Errorf("boundary assertions cannot be quantified")
			}
			switch escape {
			case 'p', 'P':
				if i+1 == len(pattern) || pattern[i+1] != '{' {
					return "", fmt.Errorf("Unicode property requires braces")
				}
				end := strings.IndexByte(pattern[i+2:], '}')
				if end < 0 {
					return "", fmt.Errorf("unterminated Unicode property")
				}
				end += i + 2
				value, err := propertyRanges(pattern[i+2:end], escape == 'P')
				if err != nil {
					return "", err
				}
				if !inClass {
					result.WriteByte('[')
				}
				result.WriteString(value)
				if !inClass {
					result.WriteByte(']')
				}
				i = end
			case 'u':
				value, end, err := unicodeEscape(pattern, i)
				if err != nil {
					return "", err
				}
				if pattern[i+1] != '{' && value >= 0xd800 && value <= 0xdbff && end+6 < len(pattern) && pattern[end+1:end+3] == `\u` && pattern[end+3] != '{' {
					low, lowEnd, lowErr := unicodeEscape(pattern, end+2)
					if lowErr == nil && low >= 0xdc00 && low <= 0xdfff {
						value = uint64(utf16.DecodeRune(rune(value), rune(low)))
						end = lowEnd
					}
				}
				fmt.Fprintf(&result, `\u{%X}`, value)
				i = end
			case 'x':
				if i+2 >= len(pattern) {
					return "", fmt.Errorf("incomplete hexadecimal escape")
				}
				if _, err := strconv.ParseUint(pattern[i+1:i+3], 16, 8); err != nil {
					return "", fmt.Errorf("invalid hexadecimal escape")
				}
				result.WriteString(pattern[i-1 : i+3])
				i += 2
			case 'c':
				if i+1 == len(pattern) || !asciiLetter(pattern[i+1]) {
					return "", fmt.Errorf("invalid control escape")
				}
				result.WriteString(pattern[i-1 : i+2])
				i++
			case '0':
				if i+1 < len(pattern) && digit(pattern[i+1]) {
					return "", fmt.Errorf("octal escapes are invalid in Unicode mode")
				}
				result.WriteString(`\0`)
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				if inClass {
					return "", fmt.Errorf("numeric escapes are invalid in Unicode character classes")
				}
				end := i + 1
				for end < len(pattern) && digit(pattern[end]) {
					end++
				}
				number, err := strconv.Atoi(pattern[i:end])
				if err != nil {
					return "", fmt.Errorf("invalid numeric backreference")
				}
				if number > maxReference {
					maxReference = number
				}
				result.WriteByte('\\')
				result.WriteString(pattern[i:end])
				i = end - 1
			default:
				allowed := strings.ContainsRune(`^$\.*+?()[]{}|/bfnrtvdDsSwW`, rune(escape)) || (!inClass && (escape == 'B' || escape == 'k')) || (inClass && escape == '-')
				if !allowed {
					return "", fmt.Errorf("invalid Unicode identity escape \\%c", escape)
				}
				result.WriteByte('\\')
				result.WriteByte(escape)
			}
			continue
		}
		if inClass {
			result.WriteByte(ch)
			if ch == ']' {
				inClass = false
			} else if ch == '^' && !classHasAtom {
				// The leading caret negates the class rather than adding an atom.
			} else if ch == '-' && classHasAtom && i+1 < len(pattern) && pattern[i+1] != ']' {
				if lastClassSet {
					return "", fmt.Errorf("character class escape cannot be a range endpoint")
				}
				rangePending = true
			} else {
				classHasAtom, lastClassSet, rangePending = true, false, false
			}
			continue
		}
		switch ch {
		case '[':
			inClass = true
			classHasAtom, lastClassSet, rangePending = false, false, false
			result.WriteByte(ch)
		case '^', '$':
			if quantifierAfter(pattern, i) {
				return "", fmt.Errorf("anchor assertions cannot be quantified")
			}
			result.WriteByte(ch)
		case ']', '}':
			return "", fmt.Errorf("unescaped %c in Unicode pattern", ch)
		case '.':
			if dotAll {
				result.WriteString(`[\s\S]`)
			} else {
				result.WriteString(`[^\n\r\u2028\u2029]`)
			}
		case '(':
			assertion := false
			if i+1 < len(pattern) && pattern[i+1] == '?' {
				remaining := pattern[i:]
				switch {
				case strings.HasPrefix(remaining, "(?="), strings.HasPrefix(remaining, "(?!"), strings.HasPrefix(remaining, "(?<="), strings.HasPrefix(remaining, "(?<!"):
					assertion = true
				case strings.HasPrefix(remaining, "(?:"):
				case strings.HasPrefix(remaining, "(?<"):
					captures++
				default:
					return "", fmt.Errorf("unsupported JavaScript group syntax")
				}
			} else {
				captures++
			}
			groups = append(groups, assertion)
			result.WriteByte(ch)
		case ')':
			if len(groups) == 0 {
				return "", fmt.Errorf("unmatched closing group")
			}
			assertion := groups[len(groups)-1]
			groups = groups[:len(groups)-1]
			if assertion && quantifierAfter(pattern, i) {
				return "", fmt.Errorf("assertions cannot be quantified in Unicode mode")
			}
			result.WriteByte(ch)
		case '{':
			end := i + 1
			for end < len(pattern) && digit(pattern[end]) {
				end++
			}
			if end == i+1 {
				return "", fmt.Errorf("invalid Unicode quantifier")
			}
			if end < len(pattern) && pattern[end] == ',' {
				end++
				for end < len(pattern) && digit(pattern[end]) {
					end++
				}
			}
			if end == len(pattern) || pattern[end] != '}' {
				return "", fmt.Errorf("unterminated quantifier")
			}
			result.WriteString(pattern[i : end+1])
			i = end
		default:
			result.WriteByte(ch)
		}
	}
	if maxReference > captures {
		return "", fmt.Errorf("backreference refers to a missing capture")
	}
	return result.String(), nil
}

func unicodeEscape(pattern string, index int) (uint64, int, error) {
	start := index + 1
	end := start + 4
	if start < len(pattern) && pattern[start] == '{' {
		start++
		offset := strings.IndexByte(pattern[start:], '}')
		if offset < 0 {
			return 0, 0, fmt.Errorf("unterminated Unicode escape")
		}
		end = start + offset
	}
	if end > len(pattern) || start == end {
		return 0, 0, fmt.Errorf("invalid Unicode escape")
	}
	value, err := strconv.ParseUint(pattern[start:end], 16, 32)
	if err != nil || value > 0x10ffff {
		return 0, 0, fmt.Errorf("invalid Unicode code point")
	}
	if pattern[index+1] == '{' {
		return value, end, nil
	}
	return value, end - 1, nil
}
func digit(ch byte) bool       { return ch >= '0' && ch <= '9' }
func asciiLetter(ch byte) bool { return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' }
func quantifierAfter(pattern string, index int) bool {
	return index+1 < len(pattern) && strings.ContainsRune("*+?{", rune(pattern[index+1]))
}
