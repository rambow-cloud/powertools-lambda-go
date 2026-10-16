package jsonvalue_test

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/internal/jsonvalue"
)

type customJSON struct {
	calls *int
	err   error
}

func (v customJSON) MarshalJSONTo(enc *jsontext.Encoder) error {
	*v.calls++
	if v.err != nil {
		return v.err
	}
	return enc.WriteValue(jsontext.Value(`{"value":12}`))
}

type namedString string

func roundTrip(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded any
	err = json.Unmarshal(data, &decoded)
	return decoded, err
}

func TestSnapshotMatchesJSONv2(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["cycle"] = cyclic
	sliceCycle := make([]any, 1)
	sliceCycle[0] = sliceCycle
	var deep any = true
	for range 80 {
		deep = []any{deep}
	}
	values := []any{
		nil, true, "text", namedString("named"), math.Copysign(0, -1), float64(0.1), float32(0.1),
		int(-123), int8(-128), int16(-32768), int32(math.MinInt32), int64(math.MinInt64),
		uint(42), uint8(255), uint16(65535), uint32(math.MaxUint32), uint64(math.MaxUint64), uintptr(123),
		jsonv1.Number("9007199254740993"), map[string]any(nil), []any(nil), []byte{1, 2},
		map[string]any{"nested": []any{map[string]any{"x": 1}, nil, []any(nil), map[string]any(nil)}},
		jsontext.Value(`{"large":9007199254740993}`), jsonv1.RawMessage(nil),
		jsontext.Value(`{"x":1,"x":2}`), jsontext.Value(`{"id":"ok","unused":1e1000}`),
		map[string]any{"id": "ok", "unused": math.NaN()}, math.Inf(1),
		string([]byte{0xff}), map[string]any{string([]byte{0xff}): true},
		make(chan int), cyclic, sliceCycle, deep,
	}
	for i, input := range values {
		want, wantErr := roundTrip(input)
		got, gotErr := jsonvalue.Snapshot(input)
		if (gotErr == nil) != (wantErr == nil) || (gotErr != nil && (reflect.TypeOf(gotErr) != reflect.TypeOf(wantErr) || gotErr.Error() != wantErr.Error())) {
			t.Fatalf("case %d errors: got %v, want %v", i, gotErr, wantErr)
		}
		if gotErr == nil && !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: got %#v, want %#v", i, got, want)
		}
	}
	got, err := jsonvalue.Snapshot(math.Copysign(0, -1))
	if err != nil || !math.Signbit(got.(float64)) {
		t.Fatal("lost negative zero")
	}
}

func TestSnapshotFallbackOnceAndOwnership(t *testing.T) {
	calls := 0
	input := map[string]any{"custom": customJSON{calls: &calls}, "rows": []any{map[string]any{"v": 1}}}
	snapshot, err := jsonvalue.Snapshot(input)
	if err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	snapshot.(map[string]any)["rows"].([]any)[0].(map[string]any)["v"] = false
	if input["rows"].([]any)[0].(map[string]any)["v"] != 1 {
		t.Fatal("aliased fallback input")
	}
	sentinel := errors.New("custom failure")
	calls = 0
	_, err = jsonvalue.Snapshot(map[string]any{"custom": customJSON{calls: &calls, err: sentinel}})
	if calls != 1 || !errors.Is(err, sentinel) {
		t.Fatal(calls, err)
	}
	shared := map[string]any{"rows": []any{map[string]any{"v": 1}}, "nil": []any(nil)}
	var wg sync.WaitGroup
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := jsonvalue.Snapshot(shared)
			if err != nil {
				t.Error(err)
				return
			}
			got.(map[string]any)["rows"].([]any)[0].(map[string]any)["v"] = false
		}()
	}
	wg.Wait()
	if shared["rows"].([]any)[0].(map[string]any)["v"] != 1 {
		t.Fatal("aliased fast input")
	}
}
