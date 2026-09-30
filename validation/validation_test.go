package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
)

type compilerFunc func(context.Context, any, CompileOptions) (Validator, error)

func (f compilerFunc) Compile(ctx context.Context, schema any, options CompileOptions) (Validator, error) {
	return f(ctx, schema, options)
}
func mustCompile(t *testing.T, schema any, options Options) *Schema {
	t.Helper()
	compiled, err := Compile(context.Background(), schema, options)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func TestCompiledOwnershipAndConcurrency(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"$ref": "https://example.test/id"}}}
	reference := map[string]any{"type": "string", "format": "id"}
	options := Options{CompileOptions: CompileOptions{ExternalRefs: map[string]any{"https://example.test/id": reference}, Formats: map[string]func(any) error{"id": func(value any) error {
		if value == "ok" {
			return nil
		}
		return fmt.Errorf("bad id")
	}}}}
	compiled := mustCompile(t, schema, options)
	reference["type"] = "boolean"
	schema["type"] = "boolean"
	options.Formats["id"] = func(any) error { return nil }
	delete(options.ExternalRefs, "https://example.test/id")
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Go(func() {
			input := map[string]any{"id": "ok"}
			output, err := compiled.Validate(context.Background(), input)
			if err != nil {
				t.Error(err)
				return
			}
			output.(map[string]any)["returnedOriginal"] = true
			if input["returnedOriginal"] != true {
				t.Error("standalone validation must return the original payload")
			}
			_, err = compiled.Validate(context.Background(), map[string]any{"id": "bad"})
			var failure *SchemaValidationError
			if !errors.As(err, &failure) {
				t.Errorf("expected format failure, got %v", err)
				return
			}
			failure.Issues[0].Params["format"] = "changed"
		})
	}
	workers.Wait()
}

func TestWrapperIsolationExtractionAndStages(t *testing.T) {
	inbound := mustCompile(t, map[string]any{"type": "object", "required": []string{"id"}}, Options{Envelope: "body"})
	outbound := mustCompile(t, map[string]any{"type": "object", "required": []string{"result"}}, Options{Envelope: "ignored"})
	input := map[string]any{"body": map[string]any{"id": "original"}}
	handler := WrapHandler[map[string]any](inbound, outbound, func(ctx context.Context, event map[string]any) (map[string]any, error) {
		event["id"] = "changed"
		return map[string]any{"result": "ok"}, nil
	})
	result, err := handler(context.Background(), input)
	if err != nil || result["result"] != "ok" {
		t.Fatalf("%v %v", result, err)
	}
	if input["body"].(map[string]any)["id"] != "original" {
		t.Fatal("handler mutated original event")
	}
	_, err = handler(context.Background(), map[string]any{"body": map[string]any{}})
	var failure *SchemaValidationError
	if !errors.As(err, &failure) || failure.Error() != "Inbound schema validation failed" || len(failure.Issues) != 1 {
		t.Fatalf("%#v", err)
	}
	bad := WrapHandler[any](nil, outbound, func(context.Context, any) (any, error) { return "bad", nil })
	_, err = bad(context.Background(), nil)
	if !errors.As(err, &failure) || failure.Error() != "Outbound schema validation failed" {
		t.Fatalf("%v", err)
	}
}

func TestWrapperTypedEventAndBusinessFailures(t *testing.T) {
	type event struct {
		Values []string `json:"values"`
	}
	inbound := mustCompile(t, true, Options{})
	original := event{Values: []string{"original"}}
	business := errors.New("business")
	handler := WrapHandler[event](inbound, inbound, func(ctx context.Context, input event) (string, error) {
		input.Values[0] = "changed"
		return "partial", business
	})
	output, err := handler(context.Background(), original)
	if output != "partial" || err != business || original.Values[0] != "original" {
		t.Fatalf("%v %v %v", output, err, original)
	}
	panicValue := &struct{}{}
	panics := WrapHandler[any](nil, inbound, func(context.Context, any) (any, error) { panic(panicValue) })
	func() {
		defer func() {
			if recover() != panicValue {
				t.Error("panic identity changed")
			}
		}()
		panics(context.Background(), nil)
	}()
	rejected := mustCompile(t, false, Options{})
	_, err = rejected.Validate(context.Background(), nil)
	if err == nil {
		t.Fatal("standalone false schema must reject")
	}
	skipped := WrapHandler[any](rejected, rejected, func(_ context.Context, value any) (any, error) { return value, nil })
	if value, err := skipped(context.Background(), "ok"); err != nil || value != "ok" {
		t.Fatalf("false wrapper schema: %v %v", value, err)
	}
}

func TestExtensionCancellationAndErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compile(ctx, true, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	operational := errors.New("validator unavailable")
	options := Options{Compiler: compilerFunc(func(context.Context, any, CompileOptions) (Validator, error) {
		return ValidatorFunc(func(context.Context, any) ([]Issue, error) { return nil, operational }), nil
	})}
	compiled := mustCompile(t, true, options)
	if _, err := compiled.Validate(context.Background(), nil); err != operational {
		t.Fatal(err)
	}
	if _, err := compiled.Validate(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	options.Compiler = compilerFunc(func(context.Context, any, CompileOptions) (Validator, error) { return nil, operational })
	_, err := Compile(context.Background(), true, options)
	var compilation *SchemaCompilationError
	if !errors.As(err, &compilation) || !errors.Is(err, operational) {
		t.Fatal(err)
	}
	options.Compiler = compilerFunc(func(context.Context, any, CompileOptions) (Validator, error) { return nil, nil })
	if _, err = Compile(context.Background(), true, options); err == nil {
		t.Fatal("nil validator accepted")
	}
	var absent *Schema
	if _, err = absent.Validate(context.Background(), nil); err == nil {
		t.Fatal("nil schema accepted")
	}
}

func TestFormatCollisionEnvelopeOptionsAndInvalidJSON(t *testing.T) {
	compiled := mustCompile(t, map[string]any{"format": "regex"}, Options{CompileOptions: CompileOptions{Formats: map[string]func(any) error{"regex": func(value any) error {
		if value == "[" {
			return nil
		}
		return errors.New("not sentinel")
	}}}})
	if _, err := compiled.Validate(context.Background(), "["); err != nil {
		t.Fatal(err)
	}
	_, err := compiled.Validate(context.Background(), "valid")
	if err == nil {
		t.Fatal("built-in regex format overrode custom callback")
	}
	extracted := mustCompile(t, map[string]any{"type": "object"}, Options{Envelope: "powertools_json(body)", QueryOptions: []jmespath.Option{jmespath.WithPowertoolsFunctions()}})
	if value, err := extracted.Validate(context.Background(), map[string]any{"body": "{\"id\":1}"}); err != nil || value == nil {
		t.Fatalf("%v %v", value, err)
	}
	standard := mustCompile(t, true, Options{Envelope: "powertools_json(body)"})
	if _, err := standard.Validate(context.Background(), map[string]any{"body": "{}"}); err == nil {
		t.Fatal("extended functions enabled by default")
	}
	if _, err := compiled.Validate(context.Background(), make(chan int)); err == nil {
		t.Fatal("non-JSON value accepted")
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	handler := WrapHandler[any](compiled, nil, func(context.Context, any) (any, error) { t.Fatal("invalid JSON reached handler"); return nil, nil })
	if _, err := handler(context.Background(), cycle); err == nil {
		t.Fatal("cycle accepted")
	}
	numbers := mustCompile(t, map[string]any{"type": "integer"}, Options{})
	value := json.Number("9007199254740993")
	if got, err := numbers.Validate(context.Background(), value); err != nil || got != value {
		t.Fatalf("large integer: %v %v", got, err)
	}
}
