package logger

import (
	"context"
	"errors"
	"io"
	"testing"
)

type testExtractor func(any) (any, error)

func (f testExtractor) Search(value any) (any, error) { return f(value) }

func TestCorrelationExtractorFailurePrecedenceAndCleanup(t *testing.T) {
	cleanEnv(t)
	want := errors.New("query failed")
	var reported error
	l := New(WithOutput(io.Discard), WithErrorHandler(func(err error) { reported = err }))
	extractor := testExtractor(func(event any) (any, error) {
		if event == "fail" {
			return nil, want
		}
		return event, nil
	})
	handler := func(ctx context.Context, event string) (any, error) {
		return l.WithContext(ctx).GetCorrelationID(), nil
	}
	wrapped := WrapHandler(l, handler, HandlerOptions{CorrelationExtractor: extractor})
	got, err := wrapped(context.Background(), "first")
	if err != nil || got != "first" {
		t.Fatal(got, err)
	}
	got, err = wrapped(context.Background(), "fail")
	if got != nil || err != nil || reported != want {
		t.Fatal(got, err, reported)
	}
	if l.GetCorrelationID() != nil {
		t.Fatal("invocation state leaked")
	}
	wrapped = WrapHandler(l, handler, HandlerOptions{CorrelationID: func(any) any { return "callback" }, CorrelationExtractor: extractor})
	got, err = wrapped(context.Background(), "fail")
	if err != nil || got != "callback" {
		t.Fatal(got, err)
	}
}
