package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
)

func TestSourceWrappersAndEmptyEnvelope(t *testing.T) {
	ctx := lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{AwsRequestID: "request"})
	p, err := NewSQS[string](Options{Sequential: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := WrapSQS(p, func(ctx context.Context, r events.SQSMessage) (string, error) {
		lc, ok := lambdacontext.FromContext(ctx)
		if !ok || lc.AwsRequestID != "request" {
			t.Error("Lambda context lost")
		}
		if _, ok := invocation.FromContext(ctx); !ok {
			t.Error("invocation context missing")
		}
		if r.MessageId == "bad" {
			return "", errors.New("bad")
		}
		return r.Body, nil
	})
	response, err := handler(ctx, events.SQSEvent{Records: []events.SQSMessage{{MessageId: "good"}, {MessageId: "bad"}}})
	if err != nil || !reflect.DeepEqual(response.BatchItemFailures, []ItemFailure{{"bad"}}) {
		t.Fatal(response, err)
	}
	for _, input := range []string{`{}`, `{"Records":null}`} {
		var event events.SQSEvent
		_ = json.Unmarshal([]byte(input), &event)
		_, err := handler(ctx, event)
		var wrong *UnexpectedBatchTypeError
		if !errors.As(err, &wrong) {
			t.Fatal(err)
		}
	}
	empty, err := handler(ctx, events.SQSEvent{Records: []events.SQSMessage{}})
	encoded, _ := json.Marshal(empty)
	if err != nil || string(encoded) != `{"batchItemFailures":[]}` {
		t.Fatal(string(encoded), err)
	}
	k, _ := NewKinesis[int](Options{SuppressFullBatchFailure: true})
	kr, err := WrapKinesis(k, func(context.Context, events.KinesisEventRecord) (int, error) { return 0, errors.New("bad") })(ctx, events.KinesisEvent{Records: []events.KinesisEventRecord{{Kinesis: events.KinesisRecord{SequenceNumber: "90071992547409930001"}}}})
	if err != nil || kr.BatchItemFailures[0].ItemIdentifier != "90071992547409930001" {
		t.Fatal(kr, err)
	}
	d, _ := NewDynamoDB[int](Options{SuppressFullBatchFailure: true})
	dr, err := WrapDynamoDB(d, func(context.Context, events.DynamoDBEventRecord) (int, error) { return 0, errors.New("bad") })(ctx, events.DynamoDBEvent{Records: []events.DynamoDBEventRecord{{Change: events.DynamoDBStreamRecord{SequenceNumber: "123"}}, {}}})
	if err != nil || !reflect.DeepEqual(dr.BatchItemFailures, []ItemFailure{{"123"}}) {
		t.Fatal(dr, err)
	}
}

func TestConcurrencyLimitsAndInvocationIsolation(t *testing.T) {
	p, _ := New[int, int](Source[int]{Identifier: func(v int) string { return fmt.Sprint(v) }}, Options{MaxConcurrency: 3})
	var active, peak atomic.Int32
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := p.Process(context.Background(), []int{1, 2, 3, 4, 5, 6}, func(_ context.Context, n int) (int, error) {
			current := active.Add(1)
			for {
				previous := peak.Load()
				if current <= previous || peak.CompareAndSwap(previous, current) {
					break
				}
			}
			if n <= 3 {
				entered <- struct{}{}
			}
			<-release
			active.Add(-1)
			return n, nil
		})
		done <- err
	}()
	for range 3 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("workers did not run concurrently")
		}
	}
	if peak.Load() != 3 {
		t.Fatal(peak.Load())
	}
	close(release)
	if err := <-done; err != nil || peak.Load() > 3 {
		t.Fatal(err, peak.Load())
	}
	var wg sync.WaitGroup
	for n := range 100 {
		wg.Go(func() {
			r, err := p.Process(context.Background(), []int{n, n + 1}, func(_ context.Context, v int) (int, error) { return v * 2, nil })
			if err != nil || len(r.Results) != 2 || r.Results[0].Result != n*2 || len(r.Failures) != 0 {
				t.Error(r, err)
			}
		})
	}
	wg.Wait()
}

func TestFullFailurePanicCancellationAndParser(t *testing.T) {
	p, _ := New[int, int](Source[int]{Identifier: func(v int) string { return fmt.Sprint(v) }}, Options{Sequential: true})
	want := errors.New("handler failed")
	report, err := p.Process(context.Background(), []int{1, 2}, func(context.Context, int) (int, error) { panic(want) })
	var full *FullBatchFailureError
	var panicError *PanicError
	if !errors.As(err, &full) || !errors.As(err, &panicError) || !errors.Is(err, want) || len(report.Errors) != 2 {
		t.Fatal(report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	visited := []int{}
	r, err := p.Process(ctx, []int{1, 2, 3}, func(_ context.Context, n int) (int, error) { visited = append(visited, n); cancel(); return n, nil })
	if err != nil || !reflect.DeepEqual(visited, []int{1}) || len(r.Failures) != 2 || !errors.Is(r.Results[1].Err, context.Canceled) || !r.Results[2].Skipped {
		t.Fatal(r, err)
	}
	parsed := WithParser(func(_ context.Context, n int) (string, error) {
		if n == 2 {
			return "", want
		}
		return fmt.Sprint(n), nil
	}, func(_ context.Context, s string) (int, error) { return len(s), nil })
	r, err = p.Process(context.Background(), []int{1, 2}, parsed)
	var parsing *ParsingError
	if err != nil || r.Results[0].Result != 1 || !errors.As(r.Results[1].Err, &parsing) || !errors.Is(r.Results[1].Err, want) || r.Response.BatchItemFailures[0].ItemIdentifier != "2" {
		t.Fatal(r, err)
	}
}

func TestFIFOStateDoesNotLeak(t *testing.T) {
	p, _ := NewSQSFIFO[int](Options{SkipGroupOnError: true, SuppressFullBatchFailure: true})
	records := []events.SQSMessage{{MessageId: "1", Attributes: map[string]string{"MessageGroupId": "a"}}, {MessageId: "2", Attributes: map[string]string{"MessageGroupId": "a"}}, {MessageId: "3", Attributes: map[string]string{"MessageGroupId": "b"}}}
	r, err := p.Process(context.Background(), records, func(_ context.Context, m events.SQSMessage) (int, error) {
		if m.MessageId == "1" {
			return 0, errors.New("bad")
		}
		return 1, nil
	})
	if err != nil || len(r.Failures) != 2 || !r.Results[1].Skipped || r.Results[2].Skipped {
		t.Fatal(r, err)
	}
	r, err = p.Process(context.Background(), records, func(context.Context, events.SQSMessage) (int, error) { return 1, nil })
	if err != nil || len(r.Successes) != 3 || len(r.Failures) != 0 {
		t.Fatal(r, err)
	}
}
