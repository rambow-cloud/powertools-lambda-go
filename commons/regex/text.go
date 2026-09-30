package regex

import (
	"unicode/utf16"
	"unicode/utf8"
)

// Decode WTF-8 as well as UTF-8 so replacement can round-trip lone surrogates.
func inputRunes(value string, unicode bool) []rune {
	var result []rune
	for len(value) > 0 {
		ch, size := utf8.DecodeRuneInString(value)
		if size == 1 && len(value) >= 3 && value[0] == 0xed && value[1] >= 0xa0 && value[1] <= 0xbf && value[2]&0xc0 == 0x80 {
			ch = rune(value[0]&15)<<12 | rune(value[1]&63)<<6 | rune(value[2]&63)
			size = 3
		}
		value = value[size:]
		if !unicode && ch > 0xffff {
			high, low := utf16.EncodeRune(ch)
			result = append(result, high, low)
		} else {
			result = append(result, ch)
		}
	}
	return result
}

func encodeRunes(value []rune) string {
	var result []byte
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch >= 0xd800 && ch <= 0xdbff && i+1 < len(value) && value[i+1] >= 0xdc00 && value[i+1] <= 0xdfff {
			ch = utf16.DecodeRune(ch, value[i+1])
			i++
		}
		if ch >= 0xd800 && ch <= 0xdfff {
			result = append(result, byte(0xe0|ch>>12), byte(0x80|(ch>>6)&63), byte(0x80|ch&63))
		} else {
			result = utf8.AppendRune(result, ch)
		}
	}
	return string(result)
}

func unitLength(value []rune) int {
	length := len(value)
	for _, ch := range value {
		if ch > 0xffff {
			length++
		}
	}
	return length
}

// JavaScript Unicode matching rewinds an offset inside a surrogate pair.
func runeOffset(value []rune, units int) int {
	for i, ch := range value {
		if units <= 0 || (units == 1 && ch > 0xffff) {
			return i
		}
		units--
		if ch > 0xffff {
			units--
		}
	}
	if units > 0 {
		return len(value) + 1
	}
	return len(value)
}
