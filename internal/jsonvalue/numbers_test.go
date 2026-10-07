package jsonvalue_test

import (
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/internal/jsonvalue"
)

func TestExactNumbersAndStrictInput(t *testing.T) {
	var value any
	if err := json.Unmarshal([]byte(`{"large":9007199254740993,"nested":[1.2300e+4,null,true,"text"]}`), &value, jsonvalue.Numbers); err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	if object["large"] != jsonv1.Number("9007199254740993") || object["nested"].([]any)[0] != jsonv1.Number("1.2300e+4") {
		t.Fatal("numeric tokens were rounded or reformatted")
	}
	for _, input := range [][]byte{
		[]byte(`{"same":1,"same":2}`),
		[]byte(`{"nested":{"same":1,"same":2}}`),
		{'"', 0xff, '"'},
		[]byte(`"\ud800"`),
		[]byte(`1 2`),
	} {
		if err := json.Unmarshal(input, &value, jsonvalue.Numbers); err == nil {
			t.Errorf("accepted invalid JSON %q", input)
		}
	}
}
