package main

import (
	"context"
	"encoding/base64"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	idempotentddb "github.com/rambow-cloud/powertools-lambda-go/idempotency/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func newKafkaProbe(tr *tracer.Tracer, log *logger.Logger, credentials aws.CredentialsProvider) func(context.Context, string) (map[string]any, error) {
	client := sdk.New(sdk.Options{Region: "ap-east-1", Credentials: credentials, BaseEndpoint: aws.String("http://capture:4318/idempotency")}, func(o *sdk.Options) { tr.InstrumentAWS(&o.APIOptions) })
	store, err := idempotentddb.New(client, idempotentddb.Options{TableName: "local-idempotency"})
	if err != nil {
		panic(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{KeyPrefix: "local-kafka", EventKeyJMESPath: "id"})
	if err != nil {
		panic(err)
	}
	schema := parser.Object(parser.Field{Name: "id", Schema: parser.String()}, parser.Field{Name: "amount", Schema: parser.Number()})
	return func(ctx context.Context, id string) (map[string]any, error) {
		parsed, executed, diagnostics := 0, 0, 0
		consumer := kafka.New(kafka.Config{Value: &kafka.FieldConfig{Type: kafka.JSON, Parser: func(ctx context.Context, input any) (kafka.ParseResult, error) {
			parsed++
			value, issues, err := schema.Validate(ctx, input)
			result := kafka.ParseResult{Value: value}
			if issues != nil {
				result.Issues = make([]any, len(issues))
				for i, issue := range issues {
					result.Issues[i] = issue
				}
			}
			return result, err
		}}, Diagnostic: func(context.Context, string, error) { diagnostics++ }})
		payload, err := json.Marshal(map[string]any{"id": id, "amount": 5})
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(payload)
		raw := jsonv1.RawMessage(fmt.Sprintf(`{"eventSource":"SelfManagedKafka","bootstrapServers":"a,b","records":{"z":[{"value":%q,"key":"","headers":[{"name":[195,169]}],"offset":2}],"a":[{"value":%q,"headers":null,"offset":1},{"value":"eyJpZCI6MSwiYW1vdW50Ijo1fQ==","headers":[]},{"value":null,"headers":[]}]}}`, encoded, encoded))
		event, err := consumer.Deserialize(ctx, raw)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"lazy": parsed == 0, "event_source": event.Fields["eventSource"], "offsets": []any{event.Records[0].Fields["offset"], event.Records[1].Fields["offset"]}}
		responses := []any{}
		for _, record := range event.Records[:2] {
			value, err := record.Value(ctx)
			if err != nil {
				return nil, err
			}
			response, err := idempotency.Execute(ctx, manager, value, func(ctx context.Context) (map[string]any, error) {
				executed++
				invocation, _ := lambdacontext.FromContext(ctx)
				return map[string]any{"id": value.(map[string]any)["id"], "request_id": invocation.AwsRequestID, "trace_id": tr.TraceID(ctx), "correlation_id": log.WithContext(ctx).GetCorrelationID()}, nil
			})
			if err != nil {
				return nil, err
			}
			responses = append(responses, response)
		}
		_, err = event.Records[2].Value(ctx)
		var invalid *kafka.ParserError
		result["parser_rejected"] = errors.As(err, &invalid) && len(invalid.Issues) == 1 && executed == 1
		tombstone, err := event.Records[3].Value(ctx)
		if err != nil {
			return nil, err
		}
		result["tombstone"] = tombstone == nil && parsed == 3
		key, err := event.Records[0].Key(ctx)
		if err != nil {
			return nil, err
		}
		_, result["empty_key"] = key.(kafka.Undefined)
		headers, err := event.Records[0].Headers(ctx)
		if err != nil {
			return nil, err
		}
		result["headers"], result["responses"], result["executions"], result["parses"], result["diagnostics"] = headers, responses, executed, parsed, diagnostics
		result["binary"], err = kafkaBinaryProbe(ctx)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
}
