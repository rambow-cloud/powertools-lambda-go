package validation

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"os"
	"sync/atomic"
	"testing"
)

func TestDefaultBoundaryReference(t *testing.T) {
	assertReference(t, "testdata/boundaries-v2.35.0.json")
}

func TestDefaultFactoryWarningObservations(t *testing.T) {
	raw, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string `json:"version"`
		AJV     string `json:"ajvVersion"`
		Cases   []struct {
			Warnings []string `json:"warnings"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || fixture.AJV != "8.20.0" || len(fixture.Cases) != 43 {
		t.Fatal("unexpected default factory corpus")
	}
	count := 0
	for _, item := range fixture.Cases {
		count += len(item.Warnings)
	}
	// Default AJV's strictTypes warnings are retained as reference evidence.
	// The Go compiler has no logger option and does not claim warning parity.
	if count != 12 {
		t.Fatalf("default strictTypes warning observations %d", count)
	}
}

func TestDefaultRegistrationBoundary(t *testing.T) {
	raw, err := os.ReadFile("testdata/registration-boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name    string `json:"name"`
			Schema  any    `json:"schema"`
			Options struct {
				ExternalRefs []any `json:"externalRefs"`
			} `json:"options"`
			Expected struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
				Message string `json:"message"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 2 {
		t.Fatal("unexpected registration observations")
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			if item.Expected.Success || item.Expected.Error != "Error" || item.Expected.Message == "" {
				t.Fatal("pinned external registration error changed")
			}
			_, err := Compile(context.Background(), item.Schema, Options{CompileOptions: CompileOptions{ExternalSchemas: item.Options.ExternalRefs}})
			var failure *SchemaCompilationError
			// AJV addSchema throws outside the compile catch. Go consistently exposes a
			// typed compilation error and retains its native cause; keep both contracts.
			if !errors.As(err, &failure) || failure.Err == nil || failure.Error() != "Failed to compile schema" {
				t.Fatalf("registration error %v", err)
			}
		})
	}
}

func TestDefaultFormatCancellationDuringEvaluation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	schema := mustCompile(t, map[string]any{"type": "string", "format": "fence"}, Options{CompileOptions: CompileOptions{Formats: map[string]func(any) error{"fence": func(any) error { calls.Add(1); close(started); <-release; return nil }}}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := schema.Validate(ctx, "value"); result <- err }()
	<-started
	cancel()
	close(release)
	if err := <-result; !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("in-flight cancel %v/%d", err, calls.Load())
	}
	if _, err := schema.Validate(ctx, "value"); !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("pre-cancel %v/%d", err, calls.Load())
	}
}

func TestDefaultReferenceEnumDiagnosticOwnership(t *testing.T) {
	target := map[string]any{"$id": "https://example.test/enum", "enum": []any{map[string]any{"name": "original"}}}
	compiled := mustCompile(t, map[string]any{"$ref": "https://example.test/enum"}, Options{CompileOptions: CompileOptions{ExternalSchemas: []any{target}}})
	target["enum"].([]any)[0].(map[string]any)["name"] = "changed"
	if _, err := compiled.Validate(context.Background(), map[string]any{"name": "original"}); err != nil {
		t.Fatal("reference snapshot changed", err)
	}
	_, err := compiled.Validate(context.Background(), nil)
	var failure *SchemaValidationError
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		t.Fatalf("enum error %v", err)
	}
	failure.Issues[0].Params["allowedValues"].([]any)[0].(map[string]any)["name"] = "changed"
	_, err = compiled.Validate(context.Background(), nil)
	if !errors.As(err, &failure) || failure.Issues[0].Params["allowedValues"].([]any)[0].(map[string]any)["name"] != "original" {
		t.Fatalf("enum diagnostics shared %v", err)
	}
}

func TestDefaultStrictNativeBoundary(t *testing.T) {
	compiled := mustCompile(t, true, Options{})
	for _, value := range []any{math.Inf(1), math.NaN(), jsontext.Value(`{"x":1,"x":2}`), jsontext.Value("\"\xff\""), jsontext.Value(`"\ud800"`)} {
		_, err := compiled.Validate(context.Background(), value)
		var failure *SchemaValidationError
		var semantic *json.SemanticError
		var syntax *jsontext.SyntacticError
		if errors.As(err, &failure) || (!errors.As(err, &semantic) && !errors.As(err, &syntax)) {
			t.Fatalf("invalid native input %T: %v", value, err)
		}
	}
}
