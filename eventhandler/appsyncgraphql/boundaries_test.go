package appsyncgraphql

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestScalarBoundaryReference(t *testing.T) {
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Millis    int64             `json:"millis"`
			Timestamp int64             `json:"timestamp"`
			Offset    any               `json:"offset"`
			Values    map[string]string `json:"values"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 45 {
		t.Fatalf("incomplete scalar boundaries: %d", len(fixture.Cases))
	}
	for i, test := range fixture.Cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			offset, ok := test.Offset.(float64)
			if !ok {
				offset = math.NaN()
			}
			now := time.UnixMilli(test.Millis)
			if got := AWSTimestamp(now); got != test.Timestamp {
				t.Fatalf("timestamp=%d, want=%d", got, test.Timestamp)
			}
			for name, fn := range map[string]func(time.Time, ...float64) (string, error){"date": AWSDate, "time": AWSTime, "datetime": AWSDateTime} {
				got, err := fn(now, offset)
				if err != nil || got != test.Values[name] {
					t.Fatalf("%s=%q, %v; want=%q", name, got, err, test.Values[name])
				}
			}
		})
	}
}

func TestScalarNativeRangeCannotWrapIntoDateDomain(t *testing.T) {
	for _, seconds := range []int64{18446744073709552, -18446744073709552} {
		now := time.Unix(seconds, 0)
		for _, test := range []struct {
			fn   func(time.Time, ...float64) (string, error)
			want string
		}{
			{AWSDate, "NaN-NaN-NaNZ"},
			{AWSTime, "NaN:NaN:NaN.NaNZ"},
			{AWSDateTime, "NaN-NaN-NaNTNaN:NaN:NaN.NaNZ"},
		} {
			got, err := test.fn(now)
			if err != nil || got != test.want {
				t.Errorf("out-of-domain instant %d wrapped: %q, %v; want %q", seconds, got, err, test.want)
			}
		}
	}
}

func TestScalarNativeNanosecondsUseUnixMillisecondFloor(t *testing.T) {
	for _, test := range []struct {
		nanos     int64
		want      string
		timestamp int64
	}{
		{1, "1970-01-01T00:00:00.000Z", 0},
		{999999, "1970-01-01T00:00:00.000Z", 0},
		{-1, "1969-12-31T23:59:59.999Z", -1},
		{-999999, "1969-12-31T23:59:59.999Z", -1},
	} {
		now := time.Unix(0, test.nanos)
		got, err := AWSDateTime(now)
		if err != nil || got != test.want || AWSTimestamp(now) != test.timestamp {
			t.Fatalf("nanoseconds=%d: %q, %v; timestamp=%d", test.nanos, got, err, AWSTimestamp(now))
		}
	}
}

func TestBatchNativeOutputBoundaries(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  []any
		bad   bool
	}{
		{"nil", nil, nil, true},
		{"nil slice", []int(nil), nil, true},
		{"scalar", 1, nil, true},
		{"map", map[string]any{}, nil, true},
		{"empty slice", []int{}, []any{}, false},
		{"empty array", [0]int{}, []any{}, false},
		{"bytes", []byte{1, 2}, []any{uint8(1), uint8(2)}, false},
		{"array", [2]any{false, nil}, []any{false, nil}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := New(quiet())
			app.OnBatchQuery("first", func(context.Context, any, any) (any, error) { return test.value, nil })
			value, err := app.Resolve(context.Background(), []Event{event("first", nil)})
			if test.bad {
				var invalid *InvalidBatchResponseException
				if value != nil || !errors.As(err, &invalid) {
					t.Fatalf("expected native framework error: %v, %v", value, err)
				}
			} else if err != nil || !reflect.DeepEqual(value, test.want) {
				t.Fatalf("native aggregate=%#v, %v; want=%#v", value, err, test.want)
			}
		})
	}
}
