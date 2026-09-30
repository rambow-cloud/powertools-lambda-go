package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSamplingTypeScriptReference(t *testing.T) {
	cleanEnv(t)
	data, err := os.ReadFile("testdata/sampling-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Draws       []int
		DrawCount   int
		Levels      []string
		Diagnostics []Fields
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	count := 0
	l := New(WithOutput(&out), WithServiceName("orders"), WithSampleRate(.1), WithRandom(func() int { value := fixture.Draws[count]; count++; return value }))
	levels := []string{l.GetLevel().String()}
	h := WrapHandler(l, func(ctx context.Context, _ int) (int, error) {
		levels = append(levels, l.WithContext(ctx).GetLevel().String())
		return 0, nil
	})
	for i := range 3 {
		_, _ = h(context.Background(), i)
	}
	diagnostics := []Fields{}
	for _, record := range records(t, &out) {
		diagnostics = append(diagnostics, Fields{"level": record["level"], "message": record["message"], "sampling_rate": record["sampling_rate"]})
	}
	if count != fixture.DrawCount || !reflect.DeepEqual(levels, fixture.Levels) || !reflect.DeepEqual(diagnostics, fixture.Diagnostics) {
		t.Fatal(count, levels, diagnostics)
	}
}

func TestConstructorSamplingAndWarmRefresh(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	draws := []int{10, 11, 0}
	count := 0
	l := New(WithOutput(&out), WithSampleRate(.1), WithRandom(func() int {
		value := draws[count]
		count++
		return value
	}))
	if count != 1 || l.GetLevel() != DebugLevel || records(t, &out)[0]["message"] != samplingMessage {
		t.Fatal("missing constructor sampling")
	}
	levels := []Level{}
	handler := WrapHandler(l, func(ctx context.Context, _ int) (int, error) {
		bound := l.WithContext(ctx)
		levels = append(levels, bound.GetLevel())
		bound.SetLevel(ErrorLevel)
		return 0, nil
	})
	for i := range 3 {
		if _, err := handler(context.Background(), i); err != nil {
			t.Fatal(err)
		}
	}
	if count != 3 || levels[0] != DebugLevel || levels[1] != InfoLevel || levels[2] != DebugLevel || l.GetLevel() != DebugLevel {
		t.Fatal(count, levels, l.GetLevel())
	}
	if len(records(t, &out)) != 2 {
		t.Fatal("expected constructor and sampled warm-invocation diagnostics", out.String())
	}
}

func TestSamplingTraceAndALC(t *testing.T) {
	for _, alc := range []string{"", "ERROR"} {
		t.Run(alc, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("AWS_LAMBDA_LOG_LEVEL", alc)
			var out bytes.Buffer
			l := New(WithOutput(&out), WithLevel(TraceLevel), WithSampleRate(1))
			want := TraceLevel
			if alc != "" {
				want = ErrorLevel
			}
			if l.GetLevel() != want || out.Len() != 0 {
				t.Fatal(l.GetLevel(), out.String())
			}
		})
	}
}
