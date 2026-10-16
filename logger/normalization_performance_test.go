package logger

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"io"
	"reflect"
	"testing"
)

type performanceEmptyString string

func (performanceEmptyString) MarshalJSONTo(enc *jsontext.Encoder) error {
	return enc.WriteToken(jsontext.String("custom"))
}

func TestFusedRootMatchesPreparedRoot(t *testing.T) {
	cyclic := Fields{}
	cyclic["again"] = cyclic
	inputs := []Fields{
		{"empty": "", "null": nil, "nil_map": map[string]any(nil), "nil_slice": []any(nil), "empty_map": Fields{}, "empty_slice": []any{}, "custom": performanceEmptyString("")},
		{"rows": []any{Fields{"name": "value"}, []string{"a", "b"}, []any(nil)}},
		cyclic,
	}
	for _, input := range inputs {
		c := New(WithOutput(io.Discard)).cfg
		data, err := encode(input, c)
		if err != nil {
			t.Fatal(err)
		}
		c.formatter = func(fields Fields) (any, error) { return fields, nil }
		reference, err := encode(input, c)
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(reference, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s, want %s", data, reference)
		}
	}
	if _, ok := inputs[0]["empty"]; !ok {
		t.Fatal("consumed caller fields")
	}
	rows := inputs[1]["rows"].([]any)
	if rows[0].(Fields)["name"] != "value" {
		t.Fatal("changed caller descendants")
	}
}

func TestSliceReplacerIndicesRemainObservable(t *testing.T) {
	seen := map[string]int{}
	c := New(WithOutput(io.Discard)).cfg
	c.replacer = func(key string, value any) any {
		if key == "0" || key == "1" {
			seen[key]++
		}
		return value
	}
	if _, err := encode(Fields{"plain": []any{"a", "b"}, "typed": []string{"c", "d"}}, c); err != nil {
		t.Fatal(err)
	}
	if seen["0"] != 2 || seen["1"] != 2 {
		t.Fatal(seen)
	}
}
