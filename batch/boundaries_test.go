package batch

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type boundaryExpectation struct {
	Response  jsontext.Value `json:"response"`
	Successes []string       `json:"successes"`
	Failures  []string       `json:"failures"`
	Results   []struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	} `json:"results"`
	Error *string `json:"error"`
}
type boundaryFixtures struct {
	Version  string `json:"version"`
	Parallel []struct {
		Source   string              `json:"source"`
		Records  []fixtureRecord     `json:"records"`
		Expected boundaryExpectation `json:"expected"`
	} `json:"parallel"`
	Warm []struct {
		Source string `json:"source"`
		FIFO   bool   `json:"fifo"`
		Steps  []struct {
			Records  []fixtureRecord     `json:"records"`
			Expected boundaryExpectation `json:"expected"`
		} `json:"steps"`
	} `json:"warm"`
	Malformed []struct {
		Source   string              `json:"source"`
		Name     string              `json:"name"`
		Records  jsontext.Value      `json:"records"`
		Expected boundaryExpectation `json:"expected"`
	} `json:"malformed"`
}

func readBoundaries(t *testing.T) boundaryFixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures boundaryFixtures
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.Version != "2.35.0" || len(fixtures.Parallel) != 3 || len(fixtures.Warm) != 4 || len(fixtures.Malformed) != 15 {
		t.Fatal("unexpected boundary corpus")
	}
	return fixtures
}
func boundarySource(source string, fifo bool) Source[fixtureRecord] {
	s := Source[fixtureRecord]{Identifier: func(r fixtureRecord) string { return r.MessageID }}
	if source == "kinesis" {
		s.Identifier = func(r fixtureRecord) string { return r.Kinesis.Sequence }
	}
	if source == "dynamodb" {
		s.Identifier = func(r fixtureRecord) string { return r.DynamoDB.Sequence }
		s.OmitEmptyIdentifier = true
	}
	if fifo {
		s.GroupID = func(r fixtureRecord) string { return r.Attributes["MessageGroupId"] }
	}
	return s
}
func assertBoundaryReport(t *testing.T, report Report[fixtureRecord, string], expected boundaryExpectation, fifo bool) {
	t.Helper()
	encoded, err := json.Marshal(report.Response)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected.Response, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response=%s want=%s", encoded, expected.Response)
	}
	successes, failures := []string{}, []string{}
	for _, r := range report.Successes {
		successes = append(successes, r.ID)
	}
	for _, r := range report.Failures {
		failures = append(failures, r.ID)
	}
	if !reflect.DeepEqual(successes, expected.Successes) || !reflect.DeepEqual(failures, expected.Failures) {
		t.Fatalf("successes=%v failures=%v want=%v %v", successes, failures, expected.Successes, expected.Failures)
	}
	if !fifo && len(report.Results) != len(expected.Results) {
		t.Fatal("input result cardinality changed")
	}
	for i, r := range expected.Results {
		if report.Results[i].Record.ID != r.ID || (report.Results[i].Err == nil) != (r.Status == "success") {
			t.Fatalf("result %d=%v want=%v", i, report.Results[i], r)
		}
	}
}

func TestParallelCompletionBoundaryReference(t *testing.T) {
	for _, row := range readBoundaries(t).Parallel {
		t.Run(row.Source, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			p, err := New[fixtureRecord, string](boundarySource(row.Source, false), Options{MaxConcurrency: 2})
			if err != nil {
				t.Fatal(err)
			}
			first, tail := make(chan struct{}), make(chan struct{})
			report, err := p.Process(ctx, row.Records, func(ctx context.Context, r fixtureRecord) (string, error) {
				switch r.ID {
				case "0":
					select {
					case <-first:
					case <-ctx.Done():
						return "", ctx.Err()
					}
				case "4":
					close(first)
					select {
					case <-tail:
					case <-ctx.Done():
						return "", ctx.Err()
					}
				case "5":
					close(tail)
				}
				if r.Fail {
					return "", fmt.Errorf("failed:%s", r.ID)
				}
				return "ok:" + r.ID, nil
			})
			// With two workers, reaching record 4 proves 1/2/3 were collected;
			// reaching 5 proves 0 was collected. The final two have different outcomes,
			// so their relative scheduling cannot change either completion list.
			if err != nil || ctx.Err() != nil {
				t.Fatalf("process=%v context=%v", err, ctx.Err())
			}
			assertBoundaryReport(t, report, row.Expected, false)
			for i, r := range report.Results {
				if r.Record.ID != row.Records[i].ID {
					t.Fatal("results lost input order")
				}
				if r.Err == nil && r.Result != "ok:"+r.Record.ID {
					t.Fatal("handler result lost")
				}
			}
			if !slices.EqualFunc(report.Response.BatchItemFailures, []ItemFailure{{"1"}, {"3"}, {"0"}, {"5"}}, func(a, b ItemFailure) bool { return a == b }) {
				t.Fatal(report.Response)
			}
		})
	}
}

func TestWarmInvocationBoundaryReference(t *testing.T) {
	for _, row := range readBoundaries(t).Warm {
		t.Run(row.Source+fmt.Sprint(row.FIFO), func(t *testing.T) {
			p, err := New[fixtureRecord, string](boundarySource(row.Source, row.FIFO), Options{Sequential: true, SuppressFullBatchFailure: true})
			if err != nil {
				t.Fatal(err)
			}
			reports := []Report[fixtureRecord, string]{}
			for _, step := range row.Steps {
				report, err := p.Process(context.Background(), step.Records, func(_ context.Context, r fixtureRecord) (string, error) {
					if r.Fail {
						return "", errors.New("failed")
					}
					return r.ID, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				assertBoundaryReport(t, report, step.Expected, row.FIFO)
				reports = append(reports, report)
			}
			// A later call must not clear slices belonging to an earlier report.
			for i, report := range reports {
				assertBoundaryReport(t, report, row.Steps[i].Expected, row.FIFO)
			}
			if row.FIFO && (!reports[0].Results[1].Skipped || len(reports[0].Results) != 2) {
				t.Fatal("Go omitted explicit FIFO skipped result")
			}
		})
	}
}

func failingLambda(source string, calls *int) lambda.Handler {
	switch source {
	case "sqs":
		p, _ := NewSQS[int](Options{SuppressFullBatchFailure: true})
		return lambda.NewHandler(WrapSQS(p, func(context.Context, events.SQSMessage) (int, error) { *calls++; return 0, errors.New("malformed") }))
	case "kinesis":
		p, _ := NewKinesis[int](Options{SuppressFullBatchFailure: true})
		return lambda.NewHandler(WrapKinesis(p, func(context.Context, events.KinesisEventRecord) (int, error) {
			*calls++
			return 0, errors.New("malformed")
		}))
	default:
		p, _ := NewDynamoDB[int](Options{SuppressFullBatchFailure: true})
		return lambda.NewHandler(WrapDynamoDB(p, func(context.Context, events.DynamoDBEventRecord) (int, error) {
			*calls++
			return 0, errors.New("malformed")
		}))
	}
}
func TestMalformedNativeLambdaBoundaryReference(t *testing.T) {
	for _, row := range readBoundaries(t).Malformed {
		t.Run(row.Source+"-"+row.Name, func(t *testing.T) {
			calls := 0
			handler := failingLambda(row.Source, &calls)
			input, err := json.Marshal(map[string]any{"Records": row.Records})
			if err != nil {
				t.Fatal(err)
			}
			response, err := handler.Invoke(context.Background(), input)
			if row.Name == "numeric-id" {
				if err == nil || calls != 0 {
					t.Fatalf("invalid native identifier executed: calls=%d error=%v", calls, err)
				}
				return
			}
			if err != nil || calls != 1 {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
			var got any
			if err := json.Unmarshal(response, &got); err != nil {
				t.Fatal(err)
			}
			expected := row.Expected.Response
			// Go native value structs turn missing/null containers into zero values;
			// unlike JS, they cannot carry undefined identifiers or property TypeErrors.
			if row.Name == "null-record" || (row.Name == "absent-id" && row.Source != "dynamodb") || (row.Name == "absent-container" && row.Source == "kinesis") {
				expected = jsontext.Value(`{"batchItemFailures":[{"itemIdentifier":""}]}`)
				if row.Source == "dynamodb" {
					expected = jsontext.Value(`{"batchItemFailures":[]}`)
				}
			}
			var want any
			if err := json.Unmarshal(expected, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native response=%s want=%s", response, expected)
			}
		})
	}
	for _, source := range []string{"sqs", "kinesis", "dynamodb"} {
		t.Run(source+"-envelopes", func(t *testing.T) {
			calls := 0
			handler := failingLambda(source, &calls)
			for _, raw := range []string{`{}`, `{"Records":null}`, `{"Records":{}}`, `{"Records":"invalid"}`, `{"Records":[7]}`} {
				if _, err := handler.Invoke(context.Background(), []byte(raw)); err == nil {
					t.Fatalf("accepted invalid envelope %s", raw)
				}
			}
			response, err := handler.Invoke(context.Background(), []byte(`{"Records":[]}`))
			if err != nil || string(response) != `{"batchItemFailures":[]}` || calls != 0 {
				t.Fatalf("response=%s calls=%d error=%v", response, calls, err)
			}
		})
	}
}

func assertParserIdentifiers[T any](t *testing.T, p *Processor[T, string], records []T, identifier func(T) string) {
	t.Helper()
	sentinel := errors.New("schema rejected record")
	calls := 0
	handler := WithParser(func(_ context.Context, r T) (string, error) {
		if identifier(r) == "90071992547409930001" {
			return "", sentinel
		}
		return "transformed", nil
	}, func(_ context.Context, value string) (string, error) { calls++; return value, nil })
	report, err := p.Process(context.Background(), records, handler)
	var parsing *ParsingError
	if err != nil || calls != 1 || !errors.As(report.Results[1].Err, &parsing) || !errors.Is(report.Results[1].Err, sentinel) || !reflect.DeepEqual(report.Response.BatchItemFailures, []ItemFailure{{"90071992547409930001"}}) || identifier(report.Results[1].Record) != "90071992547409930001" {
		t.Fatalf("report=%v calls=%d error=%v", report, calls, err)
	}
	// The same processor remains usable after the parser failure.
	again, err := p.Process(context.Background(), records[:1], handler)
	if err != nil || calls != 2 || len(again.Failures) != 0 || again.Results[0].Result != "transformed" {
		t.Fatalf("warm report=%v calls=%d error=%v", again, calls, err)
	}
}
func TestParserOriginalStreamAndMessageIdentifiers(t *testing.T) {
	t.Run("sqs", func(t *testing.T) {
		p, _ := NewSQS[string](Options{Sequential: true})
		assertParserIdentifiers(t, p, []events.SQSMessage{{MessageId: "good"}, {MessageId: "90071992547409930001"}}, func(r events.SQSMessage) string { return r.MessageId })
	})
	t.Run("kinesis", func(t *testing.T) {
		p, _ := NewKinesis[string](Options{Sequential: true})
		assertParserIdentifiers(t, p, []events.KinesisEventRecord{{Kinesis: events.KinesisRecord{SequenceNumber: "good"}}, {Kinesis: events.KinesisRecord{SequenceNumber: "90071992547409930001"}}}, func(r events.KinesisEventRecord) string { return r.Kinesis.SequenceNumber })
	})
	t.Run("dynamodb", func(t *testing.T) {
		p, _ := NewDynamoDB[string](Options{Sequential: true})
		assertParserIdentifiers(t, p, []events.DynamoDBEventRecord{{Change: events.DynamoDBStreamRecord{SequenceNumber: "good"}}, {Change: events.DynamoDBStreamRecord{SequenceNumber: "90071992547409930001"}}}, func(r events.DynamoDBEventRecord) string { return r.Change.SequenceNumber })
	})
}

func TestFIFOParserFailureAndNativeGroups(t *testing.T) {
	for _, skipGroup := range []bool{false, true} {
		t.Run(fmt.Sprint(skipGroup), func(t *testing.T) {
			p, err := NewSQSFIFO[string](Options{SkipGroupOnError: skipGroup, SuppressFullBatchFailure: true})
			if err != nil {
				t.Fatal(err)
			}
			records := []events.SQSMessage{
				{MessageId: "bad", Attributes: map[string]string{"MessageGroupId": "a"}},
				{MessageId: "same", Attributes: map[string]string{"MessageGroupId": "a"}},
				{MessageId: "other", Attributes: map[string]string{"MessageGroupId": "b"}},
			}
			sentinel := errors.New("invalid body")
			visited := []string{}
			handler := WithParser(func(_ context.Context, r events.SQSMessage) (string, error) {
				visited = append(visited, r.MessageId)
				if r.MessageId == "bad" {
					return "", sentinel
				}
				return r.MessageId, nil
			}, func(_ context.Context, value string) (string, error) { return value, nil })
			report, err := p.Process(context.Background(), records, handler)
			wantFailures := []ItemFailure{{"bad"}, {"same"}}
			wantVisited := []string{"bad", "other"}
			if !skipGroup {
				wantFailures = append(wantFailures, ItemFailure{"other"})
				wantVisited = []string{"bad"}
			}
			if err != nil || !errors.Is(report.Results[0].Err, sentinel) || !report.Results[1].Skipped || report.Results[2].Skipped == skipGroup || !reflect.DeepEqual(report.Response.BatchItemFailures, wantFailures) || !reflect.DeepEqual(visited, wantVisited) {
				t.Fatalf("report=%v visited=%v error=%v", report, visited, err)
			}
		})
	}
	t.Run("native-groups", func(t *testing.T) {
		calls := 0
		p, _ := NewSQSFIFO[int](Options{SkipGroupOnError: true, SuppressFullBatchFailure: true})
		handler := lambda.NewHandler(WrapSQS(p, func(context.Context, events.SQSMessage) (int, error) { calls++; return 1, nil }))
		for _, attributes := range []string{`{}`, `null`, `{"MessageGroupId":""}`, `{"MessageGroupId":"a"}`} {
			if _, err := handler.Invoke(context.Background(), []byte(`{"Records":[{"messageId":"m","attributes":`+attributes+`}]}`)); err != nil {
				t.Fatal(err)
			}
		}
		before := calls
		if _, err := handler.Invoke(context.Background(), []byte(`{"Records":[{"messageId":"m","attributes":{"MessageGroupId":7}}]}`)); err == nil || calls != before {
			t.Fatalf("invalid group executed: calls=%d error=%v", calls, err)
		}
	})
}
