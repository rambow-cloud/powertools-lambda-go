package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type fixtureRecord struct {
	ID         string            `json:"fixtureId"`
	Fail       bool              `json:"fail"`
	MessageID  string            `json:"messageId"`
	Attributes map[string]string `json:"attributes"`
	Kinesis    struct {
		Sequence string `json:"sequenceNumber"`
	} `json:"kinesis"`
	DynamoDB struct {
		Sequence string `json:"SequenceNumber"`
	} `json:"dynamodb"`
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, Source               string
			FIFO, Sync, Skip, Suppress bool
			Records                    []fixtureRecord
			Expected                   struct {
				Response                                 Response
				Visited, Successes, Failures, ErrorTypes []string
				Error                                    *string
			}
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			source := Source[fixtureRecord]{Identifier: func(r fixtureRecord) string { return r.MessageID }}
			if item.Source == "kinesis" {
				source.Identifier = func(r fixtureRecord) string { return r.Kinesis.Sequence }
			}
			if item.Source == "dynamodb" {
				source.Identifier = func(r fixtureRecord) string { return r.DynamoDB.Sequence }
				source.OmitEmptyIdentifier = true
			}
			if item.FIFO {
				source.GroupID = func(r fixtureRecord) string { return r.Attributes["MessageGroupId"] }
			}
			p, err := New[fixtureRecord, string](source, Options{Sequential: true, SuppressFullBatchFailure: item.Suppress, SkipGroupOnError: item.Skip})
			if err != nil {
				t.Fatal(err)
			}
			visited := []string{}
			report, err := p.Process(context.Background(), item.Records, func(_ context.Context, r fixtureRecord) (string, error) {
				visited = append(visited, r.ID)
				if r.Fail {
					return "", fmt.Errorf("failed:%s", r.ID)
				}
				return "ok:" + r.ID, nil
			})
			var full *FullBatchFailureError
			if (item.Expected.Error != nil) != errors.As(err, &full) {
				t.Fatalf("error=%v expected=%v", err, item.Expected.Error)
			}
			expectedResponse := item.Expected.Response
			if item.Expected.Error != nil {
				// Issue #53 corrects the empty full-failure response in v2.35.0.
				// Retain the upstream fixture and expect every retry identifier.
				expectedResponse = Response{BatchItemFailures: []ItemFailure{}}
				for _, record := range item.Records {
					id := source.Identifier(record)
					if id != "" || !source.OmitEmptyIdentifier {
						expectedResponse.BatchItemFailures = append(expectedResponse.BatchItemFailures, ItemFailure{ItemIdentifier: id})
					}
				}
			}
			if !reflect.DeepEqual(report.Response, expectedResponse) || !reflect.DeepEqual(visited, item.Expected.Visited) {
				t.Fatal(report.Response, visited, item.Expected)
			}
			successes, failures, errorTypes := []string{}, []string{}, []string{}
			for _, r := range report.Successes {
				successes = append(successes, r.ID)
			}
			for _, r := range report.Failures {
				failures = append(failures, r.ID)
			}
			for _, err := range report.Errors {
				name := "Error"
				switch err.(type) {
				case *FIFOShortCircuitError:
					name = "SqsFifoShortCircuitError"
				case *FIFOGroupShortCircuitError:
					name = "SqsFifoMessageGroupShortCircuitError"
				}
				errorTypes = append(errorTypes, name)
			}
			if !reflect.DeepEqual(successes, item.Expected.Successes) || !reflect.DeepEqual(failures, item.Expected.Failures) || !reflect.DeepEqual(errorTypes, item.Expected.ErrorTypes) {
				t.Fatal(successes, failures, errorTypes, item.Expected)
			}
		})
	}
}
