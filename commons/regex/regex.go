package regex

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dlclark/regexp2/v2"
)

// Options configures matching limits. Zero values use the engine defaults:
// no timeout and a 100,000-entry backtracking stack. A negative stack size
// removes that bound; a negative timeout is invalid.
type Options struct {
	MatchTimeout             time.Duration
	MaxBacktrackingStackSize int
}

// Regexp is safe for concurrent use. Stateful replacements serialize access to
// lastIndex; stateless MatchString calls can run concurrently. Do not copy it.
type Regexp struct {
	engine                         *regexp2.Regexp
	pattern                        string
	global, sticky, unicode, named bool
	legacyFold                     bool
	mu                             sync.Mutex
	lastIndex                      int
}

// Compile accepts ECMAScript d, g, i, m, s, u and y flags. The d flag has no
// effect on string replacement. Unicode set syntax (v) is not supported.
func Compile(pattern, flags string, limits Options) (*Regexp, error) {
	if limits.MatchTimeout < 0 {
		return nil, fmt.Errorf("regex match timeout must not be negative")
	}
	r := &Regexp{pattern: pattern}
	options := []regexp2.CompileOption{regexp2.ECMAScript}
	seen := map[rune]bool{}
	for _, flag := range flags {
		if seen[flag] {
			return nil, fmt.Errorf("duplicate regular expression flag %q", flag)
		}
		seen[flag] = true
		switch flag {
		case 'd':
		case 'g':
			r.global = true
		case 'y':
			r.sticky = true
		case 'u':
			r.unicode = true
			options = append(options, regexp2.Unicode)
		case 'i':
		case 'm':
			options = append(options, regexp2.Multiline)
		case 's':
		default:
			return nil, fmt.Errorf("unsupported regular expression flag %q", flag)
		}
	}
	if seen['i'] {
		if r.unicode {
			options = append(options, regexp2.IgnoreCase)
		} else {
			r.legacyFold = true
		}
	}
	var translated string
	var err error
	if r.unicode {
		translated, err = unicodePattern(pattern, seen['s'])
	} else {
		translated, err = legacyPattern(pattern, seen['s'])
	}
	if err != nil {
		return nil, err
	}
	translated = assertionsPattern(translated, seen['m'], r.unicode && seen['i'])
	if r.legacyFold {
		translated, err = foldPattern(translated)
		if err != nil {
			return nil, err
		}
	}
	if limits.MaxBacktrackingStackSize != 0 {
		options = append(options, regexp2.OptionMaxBacktrackingStackSize(limits.MaxBacktrackingStackSize))
	}
	r.engine, err = regexp2.Compile(translated, options...)
	if err != nil {
		return nil, err
	}
	if limits.MatchTimeout > 0 {
		r.engine.MatchTimeout = limits.MatchTimeout
	}
	for _, name := range r.engine.GetGroupNames() {
		if name != "" {
			if _, err := strconv.Atoi(name); err != nil {
				r.named = true
			}
		}
	}
	return r, nil
}

func (r *Regexp) String() string { return r.pattern }

// MatchString searches from the beginning without reading or changing lastIndex.
// A sticky pattern must match at the beginning.
func (r *Regexp) MatchString(input string) (bool, error) {
	match, err := r.engine.FindRunesMatch(r.matchingRunes(inputRunes(input, r.unicode)))
	return match != nil && (!r.sticky || match.RuneIndex == 0), err
}

// LastIndex returns the UTF-16 offset used by sticky, non-global replacement.
func (r *Regexp) LastIndex() int { r.mu.Lock(); defer r.mu.Unlock(); return r.lastIndex }

// SetLastIndex sets an integer UTF-16 offset. Negative values are normalized
// to zero when used by replacement, as with JavaScript's ToLength operation.
func (r *Regexp) SetLastIndex(index int) { r.mu.Lock(); defer r.mu.Unlock(); r.lastIndex = index }

// Replacer binds a replacement format for use with datamasking.Rule.Replace.
// The returned callback shares this Regexp's lastIndex.
func (r *Regexp) Replacer(format string) func(string) (string, error) {
	return func(input string) (string, error) { return r.Replace(input, format) }
}

// Replace implements JavaScript string replacement tokens and global/sticky
// state. Lone UTF-16 surrogates are preserved as WTF-8 bytes in Go strings;
// encoding/json cannot preserve those bytes as JavaScript string code units.
func (r *Regexp) Replace(input, format string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.global {
		r.lastIndex = 0
		defer func() { r.lastIndex = 0 }()
	}
	text := inputRunes(input, r.unicode)
	matching := r.matchingRunes(text)
	start := 0
	if r.sticky {
		start = runeOffset(text, max(0, r.lastIndex))
	}
	var output []rune
	copied := 0
	for start <= len(text) {
		match, err := r.engine.FindRunesMatchStartingAt(matching, start)
		if err != nil {
			return "", err
		}
		if match == nil || (r.sticky && match.RuneIndex != start) {
			if r.sticky {
				r.lastIndex = 0
			}
			break
		}
		end := match.RuneIndex + match.RuneLength
		output = append(output, text[copied:match.RuneIndex]...)
		output = append(output, r.substitute(text, format, match)...)
		copied = end
		if r.sticky {
			r.lastIndex = unitLength(text[:end])
		}
		if !r.global {
			break
		}
		start = end
		if match.RuneLength == 0 {
			start++
		}
	}
	if start > len(text) && r.sticky && !r.global {
		r.lastIndex = 0
	}
	output = append(output, text[copied:]...)
	return encodeRunes(output), nil
}

func (r *Regexp) substitute(text []rune, format string, match *regexp2.Match) []rune {
	var output []rune
	for len(format) > 0 {
		index := strings.IndexByte(format, '$')
		if index < 0 {
			return append(output, inputRunes(format, r.unicode)...)
		}
		output = append(output, inputRunes(format[:index], r.unicode)...)
		format = format[index:]
		if len(format) == 1 {
			return append(output, '$')
		}
		consumed := 2
		var capture *regexp2.Group
		switch format[1] {
		case '$':
			output = append(output, '$')
		case '&':
			output = append(output, text[match.RuneIndex:match.RuneIndex+match.RuneLength]...)
		case '`':
			output = append(output, text[:match.RuneIndex]...)
		case '\'':
			output = append(output, text[match.RuneIndex+match.RuneLength:]...)
		case '<':
			end := strings.IndexByte(format[2:], '>')
			if !r.named || end < 0 {
				consumed = 1
				output = append(output, '$')
				break
			}
			capture = match.GroupByName(format[2 : 2+end])
			consumed = end + 3
		default:
			number := int(format[1] - '0')
			if digit(format[1]) {
				if len(format) > 2 && digit(format[2]) {
					both := number*10 + int(format[2]-'0')
					if both > 0 && both < match.GroupCount() {
						number = both
						consumed++
					}
				}
				if number > 0 && number < match.GroupCount() {
					capture = match.GroupByNumber(number)
					break
				}
			}
			consumed = 1
			output = append(output, '$')
		}
		if capture != nil {
			output = append(output, text[capture.RuneIndex:capture.RuneIndex+capture.RuneLength]...)
		}
		format = format[consumed:]
	}
	return output
}
