package validation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dlclark/regexp2/v2"
)

func TestRegexOperationalErrors(t *testing.T) {
	schema := map[string]any{"type": "string", "pattern": "^(a|aa)+$"}
	compiled := mustCompile(t, schema, Options{CompileOptions: CompileOptions{Regex: RegexOptions{MaxBacktrackingStackSize: 1}}})
	_, err := compiled.Validate(context.Background(), strings.Repeat("a", 16)+"!")
	var failure *RegexError
	if !errors.As(err, &failure) || !errors.Is(err, regexp2.ErrBacktrackingStackLimit) {
		t.Fatalf("expected operational stack error, got %T: %v", err, err)
	}
	var invalid *SchemaValidationError
	if errors.As(err, &invalid) {
		t.Fatal("resource exhaustion was treated as invalid data")
	}
	wrapper := WrapHandler[any](compiled, nil, func(context.Context, any) (any, error) { t.Fatal("failed regex reached handler"); return nil, nil })
	_, err = wrapper(context.Background(), strings.Repeat("a", 16)+"!")
	if !errors.As(err, &invalid) || !errors.As(err, &failure) || invalid.Error() != "Inbound schema validation failed" {
		t.Fatalf("wrapper lost cause: %v", err)
	}
	_, err = Compile(context.Background(), schema, Options{CompileOptions: CompileOptions{Regex: RegexOptions{MatchTimeout: -time.Second}}})
	var compilation *SchemaCompilationError
	if !errors.As(err, &compilation) {
		t.Fatalf("negative timeout accepted: %v", err)
	}
}

func TestRegexTimeoutAndApplicationPanic(t *testing.T) {
	compiled := mustCompile(t, map[string]any{"pattern": "^(a|aa)+$"}, Options{CompileOptions: CompileOptions{Regex: RegexOptions{MatchTimeout: time.Millisecond, MaxBacktrackingStackSize: -1}}})
	t.Cleanup(regexp2.StopTimeoutClock)
	_, err := compiled.Validate(context.Background(), strings.Repeat("a", 35)+"!")
	var failure *RegexError
	if !errors.As(err, &failure) || !strings.Contains(failure.Err.Error(), "timeout") {
		t.Fatalf("expected timeout error: %v", err)
	}
	panicValue := &RegexError{Pattern: "application", Err: errors.New("application panic")}
	custom := mustCompile(t, map[string]any{"format": "panic"}, Options{CompileOptions: CompileOptions{Formats: map[string]func(any) error{"panic": func(any) error { panic(panicValue) }}}})
	func() {
		defer func() {
			if recover() != panicValue {
				t.Error("application panic was intercepted")
			}
		}()
		custom.Validate(context.Background(), "input")
	}()
}

func TestRegexCompiledConcurrencyAndDiagnostics(t *testing.T) {
	pattern := `^(?<word>\p{Script=Greek}+)\k<word>$`
	compiled := mustCompile(t, map[string]any{"pattern": pattern}, Options{})
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Go(func() {
			if _, err := compiled.Validate(context.Background(), "αβαβ"); err != nil {
				t.Error(err)
			}
			_, err := compiled.Validate(context.Background(), "αβ")
			var failure *SchemaValidationError
			if !errors.As(err, &failure) || len(failure.Issues) != 1 {
				t.Errorf("expected pattern failure: %v", err)
				return
			}
			if failure.Issues[0].Params["pattern"] != pattern {
				t.Error("diagnostic leaked the translated pattern")
			}
		})
	}
	workers.Wait()
}
