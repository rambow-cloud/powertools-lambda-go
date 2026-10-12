package validation

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDraft202012Reference(t *testing.T) {
	testNativeDraftReference(t, "testdata/draft-2020-12-v2.35.0.json", Draft202012Compiler{}, 56, map[string]bool{"contains-evaluated-invalid": true, "fractional-multiple-native-exact": true})
}

func testNativeDraftReference(t *testing.T, filename string, compiler Compiler, count int, differences map[string]bool) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version    string `json:"version"`
		AJVVersion string `json:"ajvVersion"`
		Cases      []struct {
			Name          string           `json:"name"`
			Schema        jsontext.Value   `json:"schema"`
			Payload       jsontext.Value   `json:"payload"`
			References    []jsontext.Value `json:"references"`
			NativeSuccess *bool            `json:"nativeSuccess"`
			NativeReason  string           `json:"nativeReason"`
			Expected      struct {
				Success bool   `json:"success"`
				Stage   string `json:"stage"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || fixture.AJVVersion != "8.20.0" || len(fixture.Cases) != count {
		t.Fatal("unexpected reference identity")
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			options := Options{Compiler: compiler}
			for _, source := range item.References {
				options.ExternalSchemas = append(options.ExternalSchemas, source)
			}
			schema, err := Compile(context.Background(), item.Schema, options)
			if item.Expected.Stage == "compilation" {
				var failure *SchemaCompilationError
				if !errors.As(err, &failure) {
					t.Fatalf("expected compile failure; got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("compile failed: %v; cause: %v", err, errors.Unwrap(err))
			}
			_, err = schema.Validate(context.Background(), item.Payload)
			success := item.Expected.Success
			if item.NativeSuccess != nil {
				if item.NativeReason == "" || !differences[item.Name] {
					t.Fatal("unreviewed native difference")
				}
				success = *item.NativeSuccess
			}
			if success {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var failure *SchemaValidationError
				if !errors.As(err, &failure) || len(failure.Issues) == 0 {
					t.Fatal("expected owned diagnostics", err)
				}
				for _, issue := range failure.Issues {
					if issue.Keyword == "" || issue.Message == "" || issue.SchemaPath == "" || issue.Params["absoluteKeywordLocation"] != issue.SchemaPath {
						t.Fatalf("incomplete native diagnostic: %+v", issue)
					}
				}
			}
		})
	}
}

func TestDraft202012SnapshotsAndConcurrency(t *testing.T) {
	root := map[string]any{"$ref": "https://example.test/library/item"}
	child := map[string]any{"$id": "item", "type": "object", "required": []any{"name"}, "properties": map[string]any{"name": map[string]any{"type": "string", "minLength": 2}}, "additionalProperties": false}
	library := map[string]any{"$id": "https://example.test/library/", "$defs": map[string]any{"item": child}}
	compiled, err := Compile(context.Background(), root, Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{ExternalSchemas: []any{library}}})
	if err != nil {
		t.Fatal(errors.Unwrap(err))
	}
	if child["$id"] != "item" {
		t.Fatal("caller schema ID was canonicalized")
	}
	root["$ref"] = "https://example.test/missing"
	child["type"] = "string"
	library["$defs"] = nil
	payload := map[string]any{"name": "Ada", "z": true, "a": true}
	var first *SchemaValidationError
	_, err = compiled.Validate(context.Background(), payload)
	if !errors.As(err, &first) || len(first.Issues) == 0 {
		t.Fatal(err)
	}
	want := fmt.Sprint(first.Issues)
	first.Issues[0].Params["absoluteKeywordLocation"] = "modified"
	first.Issues[0].Message = "modified"
	var wg sync.WaitGroup
	for index := range 48 {
		wg.Go(func() {
			if index%2 == 0 {
				if _, err := compiled.Validate(context.Background(), map[string]any{"name": "Ada"}); err != nil {
					t.Error(err)
				}
				return
			}
			_, err := compiled.Validate(context.Background(), payload)
			var failure *SchemaValidationError
			if !errors.As(err, &failure) || fmt.Sprint(failure.Issues) != want {
				t.Errorf("diagnostics changed: %v", err)
			}
		})
	}
	wg.Wait()
	if !reflect.DeepEqual(payload, map[string]any{"name": "Ada", "z": true, "a": true}) {
		t.Fatal("validation changed caller payload", payload)
	}
}

func TestDraft202012RegisteredOnlyAndResources(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	for _, source := range []any{map[string]any{"$ref": server.URL}, map[string]any{"$schema": server.URL}, jsontext.Value(`{"$defs":{"unused":{"typo":true}}}`)} {
		_, err := Compile(context.Background(), source, Options{Compiler: Draft202012Compiler{}})
		if err == nil {
			t.Fatal("unregistered resource/vocabulary compiled")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("compiler loaded a network resource")
	}
	for name, options := range map[string]CompileOptions{
		"duplicate-ID":            {ExternalSchemas: []any{map[string]any{"$id": "https://example.test/same"}, map[string]any{"$id": "https://example.test/same"}}},
		"wrong-external-dialect":  {ExternalRefs: map[string]any{"https://example.test/other": map[string]any{"$schema": "http://json-schema.org/draft-07/schema#"}}},
		"invalid-unused-document": {ExternalRefs: map[string]any{"https://example.test/other": map[string]any{"minLength": -1}}},
		"anonymous-external":      {ExternalSchemas: []any{map[string]any{"type": "string"}}},
		"fragment-resource":       {ExternalRefs: map[string]any{"https://example.test/other#part": true}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(context.Background(), true, Options{Compiler: Draft202012Compiler{}, CompileOptions: options}); err == nil {
				t.Fatal("invalid registered resource compiled")
			}
		})
	}
	compiled, err := Compile(context.Background(), map[string]any{"$ref": "https://example.test/actual#named"}, Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{ExternalRefs: map[string]any{"https://example.test/registration": map[string]any{"$id": "https://example.test/actual", "$defs": map[string]any{"value": map[string]any{"$anchor": "named", "type": "string"}}}}}})
	if err != nil {
		t.Fatal(errors.Unwrap(err))
	}
	if _, err := compiled.Validate(context.Background(), "ok"); err != nil {
		t.Fatal(err)
	}
}

func TestDraft202012FormatsCancellationAndErrors(t *testing.T) {
	ctx := context.Background()
	var compiled *Schema
	options := Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{Formats: map[string]func(any) error{"check": func(value any) error {
		if value == "outer" {
			_, err := compiled.Validate(ctx, "inner")
			return err
		}
		if value != "inner" {
			return fmt.Errorf("unexpected value")
		}
		return nil
	}}}}
	var err error
	compiled, err = Compile(ctx, map[string]any{"type": "string", "format": "check"}, options)
	if err != nil {
		t.Fatal(errors.Unwrap(err))
	}
	if _, err := compiled.Validate(ctx, "outer"); err != nil {
		t.Fatal("callback reentrancy failed", err)
	}
	if _, err := compiled.Validate(ctx, "wrong"); err == nil {
		t.Fatal("custom format was not asserted")
	}
	numbers, err := Compile(ctx, map[string]any{"type": "number", "format": "exact"}, Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{NumberFormats: map[string]func(any) error{"exact": func(value any) error {
		if value.(jsonv1.Number).String() != "9007199254740993" {
			return fmt.Errorf("wrong exact number")
		}
		return nil
	}}}})
	if err != nil {
		t.Fatal(errors.Unwrap(err))
	}
	if _, err := numbers.Validate(ctx, jsontext.Value(`9007199254740993`)); err != nil {
		t.Fatal(err)
	}
	if _, err := numbers.Validate(ctx, jsontext.Value(`9007199254740992`)); err == nil {
		t.Fatal("number format was not asserted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Compile(canceled, true, options); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := compiled.Validate(canceled, "inner"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	during, cancelDuring := context.WithCancel(ctx)
	defer cancelDuring()
	interrupt, err := Compile(ctx, map[string]any{"format": "cancel"}, Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{Formats: map[string]func(any) error{"cancel": func(any) error { cancelDuring(); return nil }}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interrupt.Validate(during, "value"); !errors.Is(err, context.Canceled) {
		t.Fatal("post-evaluation cancellation lost", err)
	}
	for _, input := range []any{math.Inf(1), jsontext.Value(`{"duplicate":1,"duplicate":2}`), jsontext.Value([]byte{'"', 0xff, '"'})} {
		_, err := compiled.Validate(ctx, input)
		var failure *SchemaValidationError
		if err == nil || errors.As(err, &failure) {
			t.Fatal("invalid JSON became schema diagnostics", err)
		}
	}
	for name, settings := range map[string]CompileOptions{
		"nil":                    {Formats: map[string]func(any) error{"check": nil}},
		"reserved":               {Formats: map[string]func(any) error{"regex": func(any) error { return nil }}},
		"duplicate":              {Formats: map[string]func(any) error{"check": func(any) error { return nil }}, NumberFormats: map[string]func(any) error{"check": func(any) error { return nil }}},
		"negative-regex-timeout": {Regex: RegexOptions{MatchTimeout: -1}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(ctx, true, Options{Compiler: Draft202012Compiler{}, CompileOptions: settings}); err == nil {
				t.Fatal("invalid compiler option accepted")
			}
		})
	}
}

func TestDraft202012DefaultRemainsDraft7(t *testing.T) {
	for _, schema := range []any{map[string]any{"$schema": draft202012URI}, map[string]any{"prefixItems": []any{true}}, map[string]any{"dependentSchemas": map[string]any{"a": true}}} {
		if _, err := Compile(context.Background(), schema, Options{}); err == nil {
			t.Fatal("default compiler expanded to a modern vocabulary")
		}
	}
	compiled, err := Compile(context.Background(), map[string]any{"type": "array", "items": []any{map[string]any{"type": "string"}}, "additionalItems": false}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiled.Validate(context.Background(), []any{"old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := compiled.Validate(context.Background(), []any{"old", 2}); err == nil {
		t.Fatal("Draft 7 tuple rule changed")
	}
}

func TestDraft202012PointerDiagnostics(t *testing.T) {
	compiled, err := Compile(context.Background(), jsontext.Value(`{"type":"object","dependentRequired":{"a b/~":["missing"]}}`), Options{Compiler: Draft202012Compiler{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiled.Validate(context.Background(), map[string]any{"a b/~": true})
	var failure *SchemaValidationError
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		t.Fatal(err)
	}
	issue := failure.Issues[0]
	if issue.InstancePath != "" || issue.Keyword != "dependentRequired" || !strings.HasSuffix(issue.SchemaPath, "/dependentRequired/a%20b~1~0") {
		t.Fatalf("wrong pointer/URI escaping: %+v", issue)
	}
}

func TestDraft202012RegexOperationalErrorAndApplicationPanic(t *testing.T) {
	compiled, err := Compile(context.Background(), map[string]any{"type": "string", "pattern": "^(a|aa)+$"}, Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{Regex: RegexOptions{MaxBacktrackingStackSize: 1}}})
	if err != nil {
		t.Fatal(errors.Unwrap(err))
	}
	_, err = compiled.Validate(context.Background(), strings.Repeat("a", 16)+"!")
	var regexError *RegexError
	var failure *SchemaValidationError
	if !errors.As(err, &regexError) || errors.As(err, &failure) {
		t.Fatal("regex exhaustion became validation diagnostics", err)
	}
	token := &struct{}{}
	custom, err := Compile(context.Background(), map[string]any{"format": "panic"}, Options{Compiler: Draft202012Compiler{}, CompileOptions: CompileOptions{Formats: map[string]func(any) error{"panic": func(any) error { panic(token) }}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() != token {
			t.Fatal("application panic was replaced")
		}
	}()
	_, _ = custom.Validate(context.Background(), "value")
}
