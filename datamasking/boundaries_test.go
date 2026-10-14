package datamasking

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"math"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestUndefinedTransformBoundaries(t *testing.T) {
	calls := 0
	masker := New(Config{Provider: providerFuncs{
		encrypt: func(context.Context, string, map[string]string) (string, error) { calls++; return "cipher", nil },
		decrypt: func(context.Context, string, map[string]string) (string, error) { calls++; return `null`, nil },
	}})
	value, err := masker.Encrypt(context.Background(), Undefined{}, TransformOptions{})
	var unsupported *Error
	if value != nil || !errors.As(err, &unsupported) || unsupported.Name != "DataMaskingUnsupportedTypeError" || calls != 0 {
		t.Errorf("undefined whole plaintext reached a string-only provider: %v, %v, calls=%d", value, err, calls)
	}
	for _, transform := range []func(context.Context, any, TransformOptions) (any, error){masker.Encrypt, masker.Decrypt} {
		value, err := transform(context.Background(), Undefined{}, TransformOptions{Fields: []string{}})
		if _, missing := value.(Undefined); !missing || err != nil || calls != 0 {
			t.Errorf("empty selection lost undefined: %v, %v, calls=%d", value, err, calls)
		}
	}
}

func TestStrictNativeCloneFailurePrecedesProviderCalls(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, input := range []any{
		math.NaN(), string([]byte{0xff}), make(chan int), cycle,
		jsonv1.RawMessage(`{"a":1,"a":2}`), jsonv1.RawMessage(`"\ud800"`),
	} {
		calls := 0
		masker := New(Config{Provider: providerFuncs{
			encrypt: func(context.Context, string, map[string]string) (string, error) { calls++; return "unexpected", nil },
			decrypt: func(context.Context, string, map[string]string) (string, error) { calls++; return `null`, nil },
		}})
		value, err := masker.Encrypt(context.Background(), input, TransformOptions{})
		var unsupported *Error
		if value != nil || !errors.As(err, &unsupported) || unsupported.Name != "DataMaskingUnsupportedTypeError" || unsupported.Cause == nil || calls != 0 {
			t.Errorf("native clone failure leaked work: input=%T, result=%v, error=%v, calls=%d", input, value, err, calls)
		}
	}
}

func TestDuplicateTransformSelectorsCaptureOriginalValues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := make(chan string, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	provider := providerFuncs{encrypt: func(ctx context.Context, plain string, _ map[string]string) (string, error) {
		calls.Add(1)
		started <- plain
		select {
		case <-release:
			return "cipher:" + plain, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	input := map[string]any{"secret": "original", "kept": []any{false, nil}}
	type result struct {
		value any
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := New(Config{Provider: provider}).Encrypt(ctx, input, TransformOptions{Fields: []string{"secret", "secret"}})
		done <- result{value, err}
	}()
	for range 2 {
		select {
		case got := <-started:
			if got != `"original"` {
				t.Errorf("overlapping selector observed a prior transform: %q", got)
			}
		case <-ctx.Done():
			t.Fatal("duplicate providers did not overlap")
		}
	}
	close(release)
	var got result
	select {
	case got = <-done:
	case <-ctx.Done():
		t.Fatal("providers did not complete")
	}
	want := map[string]any{"secret": `cipher:"original"`, "kept": []any{false, nil}}
	if got.err != nil || !reflect.DeepEqual(got.value, want) || calls.Load() != 2 {
		t.Fatalf("duplicate selectors were deduplicated or re-encrypted: %v, %v, %d", got.value, got.err, calls.Load())
	}
	got.value.(map[string]any)["kept"].([]any)[0] = "changed"
	if input["secret"] != "original" || input["kept"].([]any)[0] != false {
		t.Fatal("transform result aliases caller data")
	}
}
