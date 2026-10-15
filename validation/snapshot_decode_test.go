package validation

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"errors"
	"reflect"
	"testing"
)

type snapshotInput struct {
	calls *int
	fail  error
}

func (v snapshotInput) MarshalJSONTo(enc *jsontext.Encoder) error {
	*v.calls++
	if v.fail != nil {
		return v.fail
	}
	return enc.WriteValue(jsontext.Value(`{"large":9007199254740993,"exact":1.2300e+4}`))
}

func TestSerializedSnapshotPreservesExactNumbers(t *testing.T) {
	raw := []byte(`{"large":9007199254740993,"exact":1.2300e+4,"nested":{"id":1}}`)
	original := string(raw)
	got, err := decodeJSONValue(raw)
	if err != nil {
		t.Fatal(err)
	}
	object := got.(map[string]any)
	if object["large"] != jsonv1.Number("9007199254740993") || object["exact"] != jsonv1.Number("1.2300e+4") {
		t.Fatal(object)
	}
	object["nested"].(map[string]any)["id"] = false
	if string(raw) != original {
		t.Fatal("changed diagnostic snapshot")
	}
	reference, err := jsonValue(jsontext.Value(raw))
	if err != nil {
		t.Fatal(err)
	}
	delete(object, "nested")
	delete(reference.(map[string]any), "nested")
	if !reflect.DeepEqual(got, reference) {
		t.Fatal(got, reference)
	}
}

func TestDefaultValidatorMarshalsInputOnce(t *testing.T) {
	ctx := context.Background()
	schema, err := Compile(ctx, map[string]any{"type": "object", "properties": map[string]any{
		"large": map[string]any{"const": jsonv1.Number("9007199254740993")},
	}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	if _, err := schema.Validate(ctx, snapshotInput{calls: &calls}); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	sentinel := errors.New("input failure")
	calls = 0
	if _, err := schema.Validate(ctx, snapshotInput{calls: &calls, fail: sentinel}); !errors.Is(err, sentinel) || calls != 1 {
		t.Fatal(err, calls)
	}
}
