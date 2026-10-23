package logger

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

const maxStackFrames = 64

type errorStack struct {
	text, location, source string
	truncated              bool
}

type stackError struct {
	err   error
	stack errorStack
}

func (e *stackError) Error() string { return e.err.Error() }
func (e *stackError) Unwrap() error { return e.err }

// WithStack attaches the current call stack without changing the error's message or identity chain.
// Nil remains nil; an existing stack along a single unwrap chain is retained.
// Joined branches keep their own stacks; wrapping a join captures its aggregation site.
// This Preview is not included in v1.2.0.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := attachedStack(err); ok {
		return err
	}
	return &stackError{err: err, stack: captureStack("attached")}
}

func attachedStack(err error) (errorStack, bool) {
	// Bound traversal also handles malformed, cyclic application error chains.
	for depth := 0; err != nil && depth <= 100; depth++ {
		if wrapped, ok := err.(*stackError); ok {
			return wrapped.stack, true
		}
		single, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = single.Unwrap()
	}
	return errorStack{}, false
}

// captureStack removes leading implementation frames, retaining inlined callers
// through CallersFrames. Frame text contains no argument values or PC addresses.
func captureStack(source string) errorStack {
	// Normalization allows 100 nested properties, with two helper frames each.
	// Leave room for them without recapturing a fallback at every nested error.
	var pcs [maxStackFrames + 224]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	stack := errorStack{source: source}
	var text strings.Builder
	count := 0
	started := false
	for {
		frame, more := frames.Next()
		internal := strings.HasPrefix(frame.Function, "github.com/rambow-cloud/powertools-lambda-go/logger.") &&
			(strings.HasSuffix(frame.File, "/stack.go") || strings.HasSuffix(frame.File, "/encoding.go") ||
				strings.HasSuffix(frame.File, "/logger.go") || strings.HasSuffix(frame.File, "/lambda.go"))
		if !started && (internal || strings.HasPrefix(frame.Function, "runtime.")) {
			if !more {
				break
			}
			continue
		}
		started = true
		if !strings.HasPrefix(frame.Function, "runtime.") && !strings.HasPrefix(frame.Function, "testing.") {
			if count == maxStackFrames {
				stack.truncated = true
				break
			}
			location := frame.File + ":" + strconv.Itoa(frame.Line)
			if count == 0 {
				stack.location = location
			} else {
				text.WriteByte('\n')
			}
			text.WriteString(frame.Function)
			text.WriteString("\n\t")
			text.WriteString(location)
			count++
		}
		if !more {
			break
		}
	}
	stack.truncated = stack.truncated || n == len(pcs)
	stack.text = text.String()
	return stack
}

// panicValueError preserves a non-error panic's dynamic name in the log document.
type panicValueError struct{ value any }

func (e *panicValueError) Error() string { return fmt.Sprint(e.value) }

func errorName(err error) string {
	if value, ok := err.(*panicValueError); ok {
		return reflect.TypeOf(value.value).String()
	}
	return reflect.TypeOf(err).String()
}

// An internal stack wrapper is metadata, not a new serialized cause.
func unwrapStack(err error) error {
	for depth := 0; depth <= 100; depth++ {
		wrapped, ok := err.(*stackError)
		if !ok {
			break
		}
		err = wrapped.err
	}
	return err
}
