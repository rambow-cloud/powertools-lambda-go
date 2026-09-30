package main

import (
	"context"
	"errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rambow-cloud/powertools-lambda-go/batch"
	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

// Reuse processors across warm invocations to detect leaked failure/group state.
func newBatchProbe(tr *tracer.Tracer, log *logger.Logger) func(context.Context) (map[string]batch.Response, error) {
	sqs, _ := batch.NewSQS[string](batch.Options{Sequential: true})
	fifo, _ := batch.NewSQSFIFO[string](batch.Options{})
	groups, _ := batch.NewSQSFIFO[string](batch.Options{SkipGroupOnError: true})
	kinesis, _ := batch.NewKinesis[string](batch.Options{Sequential: true})
	dynamodb, _ := batch.NewDynamoDB[string](batch.Options{Sequential: true})
	query := jmespath.MustCompile("powertools_json(body).id", jmespath.WithPowertoolsFunctions())
	parsed := batch.WithParser(func(_ context.Context, record events.SQSMessage) (string, error) {
		value, err := query.Search(record)
		if err != nil {
			return "", err
		}
		id, ok := value.(string)
		if !ok {
			return "", errors.New("record id is required")
		}
		return id, nil
	}, func(ctx context.Context, id string) (string, error) {
		return tracer.Capture(ctx, tr, "batch-record", func(ctx context.Context) (string, error) {
			if err := log.WithContext(ctx).Info("batch record", logger.Fields{"record_id": id}); err != nil {
				return "", err
			}
			if id == "bad" {
				return "", errors.New("intentional record failure")
			}
			return id, nil
		})
	})
	return func(ctx context.Context) (map[string]batch.Response, error) {
		result := map[string]batch.Response{}
		for _, item := range []struct {
			name string
			p    batch.RecordProcessor[events.SQSMessage, string]
		}{{"sqs", sqs}, {"fifo", fifo}, {"fifo_groups", groups}} {
			records := []events.SQSMessage{
				{MessageId: "1", Body: `{"id":"good"}`, Attributes: map[string]string{"MessageGroupId": "a"}},
				{MessageId: "2", Body: `{"id":"bad"}`, Attributes: map[string]string{"MessageGroupId": "a"}},
				{MessageId: "3", Body: `{"id":"good"}`, Attributes: map[string]string{"MessageGroupId": "b"}},
				{MessageId: "4", Body: `{"id":"good"}`, Attributes: map[string]string{"MessageGroupId": "a"}},
			}
			if item.name == "sqs" {
				records[1].Body = "invalid json"
			}
			response, err := batch.WrapSQS(item.p, parsed)(ctx, events.SQSEvent{Records: records})
			if err != nil {
				return nil, err
			}
			result[item.name] = response
		}
		kr, err := batch.WrapKinesis(kinesis, func(_ context.Context, r events.KinesisEventRecord) (string, error) {
			if r.Kinesis.SequenceNumber == "90071992547409930002" {
				return "", errors.New("retry")
			}
			return "ok", nil
		})(ctx, events.KinesisEvent{Records: []events.KinesisEventRecord{{Kinesis: events.KinesisRecord{SequenceNumber: "90071992547409930001"}}, {Kinesis: events.KinesisRecord{SequenceNumber: "90071992547409930002"}}}})
		if err != nil {
			return nil, err
		}
		result["kinesis"] = kr
		dr, err := batch.WrapDynamoDB(dynamodb, func(_ context.Context, r events.DynamoDBEventRecord) (string, error) {
			if r.Change.SequenceNumber == "90071992547409930002" {
				return "", errors.New("retry")
			}
			return "ok", nil
		})(ctx, events.DynamoDBEvent{Records: []events.DynamoDBEventRecord{{Change: events.DynamoDBStreamRecord{SequenceNumber: "90071992547409930001"}}, {Change: events.DynamoDBStreamRecord{SequenceNumber: "90071992547409930002"}}}})
		if err != nil {
			return nil, err
		}
		result["dynamodb"] = dr
		return result, nil
	}
}
