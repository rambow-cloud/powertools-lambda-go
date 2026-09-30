package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"POWERTOOLS_LOG_LEVEL", "LOG_LEVEL", "AWS_LAMBDA_LOG_LEVEL", "POWERTOOLS_LOGGER_SAMPLE_RATE", "POWERTOOLS_DEV", "POWERTOOLS_LOGGER_LOG_EVENT", "TZ", "_X_AMZN_TRACE_ID", "POWERTOOLS_SERVICE_NAME"} {
		t.Setenv(k, "")
	}
}
func records(t *testing.T, b *bytes.Buffer) []Fields {
	t.Helper()
	var out []Fields
	for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
		if line == "" {
			continue
		}
		var f Fields
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			t.Fatal(err, line)
		}
		out = append(out, f)
	}
	return out
}
func TestRecordPrecedenceAndEncoding(t *testing.T) {
	cleanEnv(t)
	var b bytes.Buffer
	l := New(WithOutput(&b), WithServiceName("orders"), WithClock(func() time.Time { return time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC) }), WithPersistentKeys(Fields{"order": "base"}))
	l.AppendKeys(Fields{"order": "temporary"})
	if err := l.Info("created", Fields{"order": "call", "html": "<ok>"}); err != nil {
		t.Fatal(err)
	}
	want := `{"level":"INFO","message":"created","timestamp":"2026-09-12T00:00:00.000Z","service":"orders","sampling_rate":0,"html":"<ok>","order":"call"}` + "\n"
	if b.String() != want {
		t.Fatalf("got %s, want %s", b.String(), want)
	}
	b.Reset()
	_ = l.Info("safe", Fields{"message": "override"})
	r := records(t, &b)
	if len(r) != 2 || r[1]["message"] != "safe" {
		t.Fatal(r)
	}
	cycle := Fields{}
	cycle["self"] = cycle
	b.Reset()
	if err := l.Info("cycle", Fields{"cycle": cycle}, fmt.Errorf("outer: %w", errors.New("inner"))); err != nil {
		t.Fatal(err)
	}
	r = records(t, &b)
	if r[0]["cycle"].(map[string]any)["self"] != "[Circular]" || r[0]["error"].(map[string]any)["cause"] == nil {
		t.Fatal(r)
	}
	if err := l.Info("invalid", Fields{"channel": make(chan int)}); err == nil {
		t.Fatal("expected encoding error")
	}
}
func TestALCAndSampling(t *testing.T) {
	cleanEnv(t)
	t.Setenv("AWS_LAMBDA_LOG_LEVEL", "ERROR")
	var b bytes.Buffer
	l := New(WithOutput(&b), WithLevel(TraceLevel), WithSampleRate(1))
	l.SetLevel(DebugLevel)
	_ = l.Info("hidden")
	_ = l.Error("visible")
	if len(records(t, &b)) != 1 || l.GetLevel() != ErrorLevel {
		t.Fatal(b.String())
	}
	t.Setenv("AWS_LAMBDA_LOG_LEVEL", "")
	b.Reset()
	l = New(WithOutput(&b), WithSampleRate(.1), WithRandom(func() int { return 10 }))
	h := WrapHandler(l, func(ctx context.Context, _ int) (int, error) { return 1, l.WithContext(ctx).Debug("sampled") })
	if _, err := h(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if len(records(t, &b)) != 2 || l.GetLevel() != DebugLevel {
		t.Fatal("constructor sampling or first-invocation reuse failed")
	}
}
func TestInvocationIsolationAndChildren(t *testing.T) {
	cleanEnv(t)
	var b, childOutput bytes.Buffer
	l := New(WithOutput(&b), WithPersistentKeys(Fields{"shared": true}))
	var retained *Logger
	h := WrapHandler(l, func(ctx context.Context, n int) (int, error) {
		bound := l.WithContext(ctx)
		retained = bound
		if n == 1 {
			bound.AppendKeys(Fields{"temporary": 1})
			bound.AppendPersistentKeys(Fields{"request_only": true})
		}
		if err := bound.Info("request"); err != nil {
			return 0, err
		}
		child := bound.Child(WithOutput(&childOutput), WithPersistentKeys(Fields{"child": true}))
		return n, child.Info("child")
	})
	for n := 1; n <= 2; n++ {
		ctx := lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{AwsRequestID: fmt.Sprint(n)})
		if got, err := h(ctx, n); got != n || err != nil {
			t.Fatal(got, err)
		}
	}
	r := records(t, &b)
	if r[0]["temporary"] == nil || r[1]["temporary"] != nil || r[1]["request_only"] != nil || r[1]["function_request_id"] != "2" {
		t.Fatal(r)
	}
	children := records(t, &childOutput)
	if len(children) != 2 || children[1]["shared"] != true || children[1]["child"] != true {
		t.Fatal(children)
	}
	if !errors.Is(retained.Info("late"), ErrInvocationClosed) {
		t.Fatal("late write accepted")
	}
}
func TestConcurrentInvocations(t *testing.T) {
	cleanEnv(t)
	var b bytes.Buffer
	l := New(WithOutput(&b))
	h := WrapHandler(l, func(ctx context.Context, n int) (int, error) {
		bound := l.WithContext(ctx)
		bound.AppendKeys(Fields{"request": n})
		return n, bound.Info(fmt.Sprint(n))
	})
	var wg sync.WaitGroup
	for n := 0; n < 100; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h(context.Background(), n); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	r := records(t, &b)
	if len(r) != 100 {
		t.Fatal(len(r))
	}
	for _, f := range r {
		if f["message"] != fmt.Sprint(f["request"]) {
			t.Fatal(f)
		}
	}
}
func TestBufferLifecycle(t *testing.T) {
	cleanEnv(t)
	t.Setenv("_X_AMZN_TRACE_ID", "Root=1-12345678-123456789012345678901234;Parent=1234567890123456;Sampled=1")
	var b bytes.Buffer
	l := New(WithOutput(&b), WithBuffer(BufferOptions{}))
	_ = l.Debug("buffered")
	if b.Len() != 0 {
		t.Fatal("debug was not buffered")
	}
	_ = l.Error("failure")
	r := records(t, &b)
	if len(r) != 2 || r[0]["message"] != "buffered" {
		t.Fatal(r)
	}
	b.Reset()
	l = New(WithOutput(&b), WithBuffer(BufferOptions{MaxBytes: 400}))
	for n := 0; n < 10; n++ {
		_ = l.Debug(fmt.Sprint(n))
	}
	_ = l.FlushBuffer()
	r = records(t, &b)
	if len(r) < 2 || !strings.Contains(r[len(r)-1]["message"].(string), "evicted") {
		t.Fatal(r)
	}
	b.Reset()
	l = New(WithOutput(&b), WithBuffer(BufferOptions{MaxBytes: 10}))
	_ = l.Debug("oversize")
	if len(records(t, &b)) != 2 {
		t.Fatal(b.String())
	}
	b.Reset()
	l = New(WithOutput(&b), WithBuffer(BufferOptions{}))
	sentinel := errors.New("business error")
	h := WrapHandler(l, func(ctx context.Context, n int) (int, error) {
		_ = l.WithContext(ctx).Debug("detail")
		if n == 1 {
			return n, nil
		}
		return n, sentinel
	}, HandlerOptions{FlushBufferOnError: true})
	_, _ = h(context.Background(), 1)
	if b.Len() != 0 {
		t.Fatal("successful invocation flushed")
	}
	got, err := h(context.Background(), 2)
	if got != 2 || err != sentinel || len(records(t, &b)) != 2 {
		t.Fatal(got, err, b.String())
	}
}
func TestPanicPreserved(t *testing.T) {
	cleanEnv(t)
	var b bytes.Buffer
	l := New(WithOutput(&b))
	token := &struct{}{}
	h := WrapHandler(l, func(context.Context, int) (int, error) { panic(token) })
	defer func() {
		if recover() != token {
			t.Error("panic changed")
		}
	}()
	_, _ = h(context.Background(), 0)
}
