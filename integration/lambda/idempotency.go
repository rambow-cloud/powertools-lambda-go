package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/batch"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	idempotentddb "github.com/rambow-cloud/powertools-lambda-go/idempotency/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type idempotencyResult struct {
	WarmExecutions     int32          `json:"warm_executions"`
	WarmResponse       int32          `json:"warm_response"`
	RecordExecutions   int            `json:"record_executions"`
	Batch              batch.Response `json:"batch"`
	Retry              batch.Response `json:"retry"`
	ValidationRejected bool           `json:"validation_rejected"`
	ParserRejected     bool           `json:"parser_rejected"`
}
type order struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

func newIdempotencyProbe(tr *tracer.Tracer, l *logger.Logger, credentials aws.CredentialsProvider) func(context.Context, string) (idempotencyResult, error) {
	client := sdk.New(sdk.Options{Region: "ap-east-1", Credentials: credentials, BaseEndpoint: aws.String("http://capture:4318/idempotency")}, func(o *sdk.Options) { tr.InstrumentAWS(&o.APIOptions) })
	store, err := idempotentddb.New(client, idempotentddb.Options{TableName: "local-idempotency"})
	if err != nil {
		panic(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{KeyPrefix: "local-orders", EventKeyJMESPath: "id", PayloadValidationJMESPath: "amount"})
	if err != nil {
		panic(err)
	}
	processor, _ := batch.NewSQS[int](batch.Options{Sequential: true})
	var warmExecutions atomic.Int32
	return func(ctx context.Context, id string) (idempotencyResult, error) {
		result := idempotencyResult{}
		warm, err := idempotency.Execute(ctx, manager, order{ID: "warm", Amount: 1}, func(context.Context) (int32, error) { return warmExecutions.Add(1), nil })
		if err != nil {
			return result, err
		}
		result.WarmResponse = warm
		result.WarmExecutions = warmExecutions.Load()
		calls := 0
		failed := false
		handle := idempotency.WrapHandler(manager, func(ctx context.Context, input order) (int, error) {
			return tracer.Capture(ctx, tr, "idempotent-record", func(ctx context.Context) (int, error) {
				calls++
				if err := l.WithContext(ctx).Info("idempotent record", logger.Fields{"record_id": input.ID}); err != nil {
					return 0, err
				}
				if input.ID == id+"-b" && !failed {
					failed = true
					return 0, errors.New("intentional idempotent record failure")
				}
				return calls, nil
			})
		})
		parsed := batch.WithParser(func(ctx context.Context, message events.SQSMessage) (order, error) {
			return parser.Parse(ctx, message.Body, parser.JSONStringified(orderInputSchema))
		}, handle)
		_, parseErr := parsed(ctx, events.SQSMessage{Body: `{"id":1,"amount":-1}`})
		var parseFailure *parser.ParseError
		result.ParserRejected = errors.As(parseErr, &parseFailure) && len(parseFailure.Issues) == 2 && calls == 0
		if !result.ParserRejected {
			return result, errors.New("invalid order reached idempotency")
		}
		message := func(messageID, orderID string) events.SQSMessage {
			body, _ := json.Marshal(order{ID: orderID, Amount: 1})
			return events.SQSMessage{MessageId: messageID, Body: string(body)}
		}
		retry := message("3", id+"-b")
		result.Batch, err = batch.WrapSQS(processor, parsed)(ctx, events.SQSEvent{Records: []events.SQSMessage{message("1", id+"-a"), message("2", id+"-a"), retry}})
		if err != nil {
			return result, err
		}
		result.Retry, err = batch.WrapSQS(processor, parsed)(ctx, events.SQSEvent{Records: []events.SQSMessage{retry}})
		if err != nil {
			return result, err
		}
		_, err = handle(ctx, order{ID: id + "-a", Amount: 2})
		var validation *idempotency.ValidationError
		if !errors.As(err, &validation) {
			return result, errors.New("changed payload was not rejected")
		}
		result.ValidationRejected = true
		result.RecordExecutions = calls
		return result, nil
	}
}
