package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestApplicatorConcurrentDiagnostics(t *testing.T) {
	schema := mustCompile(t, json.RawMessage(`{"properties":{"zebra":{"if":{"required":["card"]},"then":{"dependencies":{"card":["zip","name"]}},"propertyNames":{"pattern":"^[a-z]+$"}},"alpha":{"items":[{"type":"integer"}],"additionalItems":false}}}`), Options{})
	payload := json.RawMessage(`{"alpha":[false,2],"zebra":{"card":true,"BAD/~":1}}`)
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Go(func() {
			_, err := schema.Validate(context.Background(), payload)
			var failure *SchemaValidationError
			if !errors.As(err, &failure) {
				t.Errorf("expected validation failure: %v", err)
				return
			}
			var keywords, paths []string
			for _, issue := range failure.Issues {
				keywords = append(keywords, issue.Keyword)
				paths = append(paths, issue.InstancePath)
			}
			if !reflect.DeepEqual(keywords, []string{"dependencies", "dependencies", "if", "pattern", "propertyNames", "additionalItems", "type"}) || !reflect.DeepEqual(paths, []string{"/zebra", "/zebra", "/zebra", "/zebra", "/zebra", "/alpha", "/alpha/0"}) {
				t.Errorf("unexpected scoped order: %v %v", keywords, paths)
			}
			if len(failure.Issues) == 7 {
				*failure.Issues[3].PropertyName = "mutated"
				failure.Issues[0].Params["deps"] = "mutated"
			}
		})
	}
	workers.Wait()
}

func TestOneOfEvaluatesEachVisitedBranchOnce(t *testing.T) {
	var calls []string
	formats := map[string]func(any) error{}
	for _, name := range []string{"first", "fail", "second", "unvisited"} {
		formats[name] = func(any) error {
			calls = append(calls, name)
			if name == "fail" {
				return fmt.Errorf("rejected")
			}
			return nil
		}
	}
	schema := mustCompile(t, json.RawMessage(`{"oneOf":[{"format":"first"},{"format":"fail"},{"format":"second"},{"format":"unvisited"}]}`), Options{CompileOptions: CompileOptions{Formats: formats}})
	_, err := schema.Validate(context.Background(), "value")
	var failure *SchemaValidationError
	if !errors.As(err, &failure) || len(failure.Issues) != 2 || failure.Issues[0].Keyword != "format" {
		t.Fatalf("diagnostics: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"first", "fail", "second"}) {
		t.Fatalf("branch visits: %v", calls)
	}
}

func TestDiagnosticValuesDoNotMutateCompiledSchema(t *testing.T) {
	schema := mustCompile(t, json.RawMessage(`{"type":["object","array"],"const":{"nested":["safe"]},"enum":[{"nested":["safe"]}]}`), Options{})
	_, err := schema.Validate(context.Background(), false)
	var first *SchemaValidationError
	if !errors.As(err, &first) {
		t.Fatal(err)
	}
	want, _ := json.Marshal(first.Issues)
	first.Issues[0].Params["type"].([]any)[0] = "boolean"
	first.Issues[1].Params["allowedValue"].(map[string]any)["nested"].([]any)[0] = "changed"
	first.Issues[2].Params["allowedValues"].([]any)[0].(map[string]any)["nested"].([]any)[0] = "changed"
	_, err = schema.Validate(context.Background(), false)
	var second *SchemaValidationError
	if !errors.As(err, &second) {
		t.Fatal(err)
	}
	got, _ := json.Marshal(second.Issues)
	if string(got) != string(want) {
		t.Fatalf("caller mutated schema diagnostics: %s", got)
	}
	if _, err = schema.Validate(context.Background(), map[string]any{"nested": []string{"safe"}}); err != nil {
		t.Fatalf("caller mutated validation: %v", err)
	}
}

type countedJSON struct {
	calls *int
	raw   string
}

func (v countedJSON) MarshalJSON() ([]byte, error) { *v.calls++; return []byte(v.raw), nil }

func TestDiagnosticOrderUsesOneSerializationSnapshot(t *testing.T) {
	var schemaCalls, payloadCalls, referenceCalls int
	schema := mustCompile(t, countedJSON{&schemaCalls, `{"$ref":"https://example.test/order"}`}, Options{CompileOptions: CompileOptions{ExternalRefs: map[string]any{"https://example.test/order": countedJSON{&referenceCalls, `{"properties":{"zebra":{"type":"integer"},"alpha":{"type":"integer"}}}`}}}})
	_, err := schema.Validate(context.Background(), countedJSON{&payloadCalls, `{"alpha":false,"zebra":false}`})
	if err == nil || schemaCalls != 1 || payloadCalls != 1 || referenceCalls != 1 {
		t.Fatalf("error %v; serialization counts %d/%d/%d", err, schemaCalls, payloadCalls, referenceCalls)
	}
}
