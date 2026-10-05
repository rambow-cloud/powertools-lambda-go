package batch

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func checkFullFailureResponse[T any](t *testing.T, processor *Processor[T, string], records []T, suppress bool, identifiers []string) {
	t.Helper()
	cause := errors.New("record failed")
	handler := func(context.Context, T) (string, error) { return "", cause }
	report, err := processor.Process(context.Background(), records, handler)
	var full *FullBatchFailureError
	if suppress {
		if err != nil {
			t.Fatal(err)
		}
	} else if !errors.As(err, &full) || !errors.Is(err, cause) || len(full.RecordErrors) != len(records) {
		t.Fatalf("full batch error or record causes were lost: %v", err)
	}
	if len(report.Failures) != len(records) || len(report.Successes) != 0 {
		t.Fatalf("failures=%d successes=%d", len(report.Failures), len(report.Successes))
	}
	assertIdentifiers := func(response Response) {
		t.Helper()
		got := make([]string, 0, len(response.BatchItemFailures))
		for _, failure := range response.BatchItemFailures {
			got = append(got, failure.ItemIdentifier)
		}
		slices.Sort(got)
		if !reflect.DeepEqual(got, identifiers) {
			t.Fatalf("failure identifiers=%v want=%v", got, identifiers)
		}
	}
	assertIdentifiers(report.Response)
	// Lambda wrappers must expose the same populated response alongside the error.
	wrapper := WrapHandler(processor, func(records []T) []T { return records }, handler)
	response, err := wrapper(context.Background(), records)
	if suppress && err != nil || !suppress && !errors.As(err, &full) {
		t.Fatalf("wrapper error=%v", err)
	}
	assertIdentifiers(response)
}

func TestFullFailureResponseAcrossSources(t *testing.T) {
	for _, sequential := range []bool{false, true} {
		for _, suppress := range []bool{false, true} {
			options := Options{Sequential: sequential, SuppressFullBatchFailure: suppress}
			t.Run(fmt.Sprintf("sequential=%t/suppress=%t", sequential, suppress), func(t *testing.T) {
				t.Run("sqs", func(t *testing.T) {
					processor, err := NewSQS[string](options)
					if err != nil {
						t.Fatal(err)
					}
					checkFullFailureResponse(t, processor, []events.SQSMessage{{MessageId: "a"}, {MessageId: "b"}}, suppress, []string{"a", "b"})
				})
				t.Run("fifo", func(t *testing.T) {
					processor, err := NewSQSFIFO[string](options)
					if err != nil {
						t.Fatal(err)
					}
					records := []events.SQSMessage{{MessageId: "a", Attributes: map[string]string{"MessageGroupId": "group"}}, {MessageId: "b", Attributes: map[string]string{"MessageGroupId": "group"}}}
					checkFullFailureResponse(t, processor, records, suppress, []string{"a", "b"})
				})
				t.Run("kinesis", func(t *testing.T) {
					processor, err := NewKinesis[string](options)
					if err != nil {
						t.Fatal(err)
					}
					records := []events.KinesisEventRecord{{Kinesis: events.KinesisRecord{SequenceNumber: "a"}}, {Kinesis: events.KinesisRecord{SequenceNumber: "b"}}}
					checkFullFailureResponse(t, processor, records, suppress, []string{"a", "b"})
				})
				t.Run("dynamodb", func(t *testing.T) {
					processor, err := NewDynamoDB[string](options)
					if err != nil {
						t.Fatal(err)
					}
					records := []events.DynamoDBEventRecord{{Change: events.DynamoDBStreamRecord{SequenceNumber: "a"}}, {Change: events.DynamoDBStreamRecord{SequenceNumber: "b"}}, {Change: events.DynamoDBStreamRecord{SequenceNumber: ""}}}
					checkFullFailureResponse(t, processor, records, suppress, []string{"a", "b"})
				})
			})
		}
	}
}
