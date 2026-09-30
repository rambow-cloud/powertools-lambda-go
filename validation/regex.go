package validation

import (
	"fmt"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/regex"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// RegexOptions configures operational limits without changing pattern syntax.
// Zero values use the engine's defaults: no timeout and a 100,000-entry stack.
type RegexOptions struct {
	MatchTimeout time.Duration
	// A positive value sets the stack bound; a negative value removes it.
	MaxBacktrackingStackSize int
}

func (o RegexOptions) validate() error {
	if o.MatchTimeout < 0 {
		return fmt.Errorf("regex match timeout must not be negative")
	}
	return nil
}

func (o RegexOptions) compile(pattern string) (jsonschema.Regexp, error) {
	compiled, err := regex.Compile(pattern, "u", o.shared())
	if err != nil {
		return nil, err
	}
	return &ecmaRegexp{Regexp: compiled, pattern: pattern}, nil
}

func (o RegexOptions) shared() regex.Options {
	return regex.Options{MatchTimeout: o.MatchTimeout, MaxBacktrackingStackSize: o.MaxBacktrackingStackSize}
}

type ecmaRegexp struct {
	*regex.Regexp
	pattern string
}

func (r *ecmaRegexp) String() string { return r.pattern }

func (r *ecmaRegexp) MatchString(value string) bool {
	matched, err := r.Regexp.MatchString(value)
	if err != nil {
		panic(regexAbort{&RegexError{Pattern: r.String(), Err: err}})
	}
	return matched
}

// RegexError identifies an operational match failure, not an invalid payload.
type RegexError struct {
	Pattern string
	Err     error
}

func (e *RegexError) Error() string {
	return fmt.Sprintf("regular expression %q failed: %v", e.Pattern, e.Err)
}
func (e *RegexError) Unwrap() error { return e.Err }

// The schema engine's regex interface has no error return. Only this private
// panic transports matching errors across it; application panics are untouched.
type regexAbort struct{ err *RegexError }

func recoverRegexError(err *error) {
	if value := recover(); value != nil {
		if failure, ok := value.(regexAbort); ok {
			*err = failure.err
			return
		}
		panic(value)
	}
}
