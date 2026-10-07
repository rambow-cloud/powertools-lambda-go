package main

import (
	"context"
	json "encoding/json/v2"
	"errors"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func appSyncEventsProbe(ctx context.Context, log *logger.Logger, tr *tracer.Tracer) (map[string]any, error) {
	var input map[string]any
	if err := json.Unmarshal([]byte(`{"identity":null,"result":null,"request":{"headers":{},"domainName":null},"error":null,"prev":null,"stash":{},"outErrors":[],"events":[{"id":"one","payload":{"id":"order-a"}},{"id":"two","payload":{"id":"order-b"}}],"info":{"channel":{"path":"/default/orders","segments":["default","orders"]},"channelNamespace":{"name":"default"},"operation":"PUBLISH"}}`), &input); err != nil {
		return nil, err
	}
	if _, err := parser.Parse(ctx, input, schemas.AppSyncEventsPublishSchema); err != nil {
		return nil, err
	}
	app := appsyncevents.New(appsyncevents.Options{Diagnostic: func(context.Context, string, string, error) {}})
	app.OnPublish("/default/*", func(ctx context.Context, value any, event appsyncevents.Event) (any, error) {
		invocation, _ := lambdacontext.FromContext(ctx)
		return map[string]any{"order": value, "request_id": invocation.AwsRequestID, "trace_id": tr.TraceID(ctx), "correlation_id": log.WithContext(ctx).GetCorrelationID()}, nil
	})
	app.OnPublish("/default/aggregate", func(ctx context.Context, value any, event appsyncevents.Event) (any, error) { return value, nil }, appsyncevents.PublishOptions{Aggregate: true})
	app.OnPublish("/default/error", func(context.Context, any, appsyncevents.Event) (any, error) {
		return nil, errors.New("denied")
	})
	denied := &appsyncevents.UnauthorizedError{Message: "denied"}
	app.OnSubscribe("/default/deny", func(context.Context, appsyncevents.Event) error { return denied })
	result := map[string]any{"parsed": true}
	info := input["info"].(map[string]any)
	channel := info["channel"].(map[string]any)
	for _, item := range []struct{ key, path string }{{"individual", "/default/orders"}, {"aggregate", "/default/aggregate"}, {"item_errors", "/default/error"}, {"passthrough", "/missing"}} {
		channel["path"] = item.path
		value, err := app.Resolve(ctx, input)
		if err != nil {
			return nil, err
		}
		result[item.key] = value
	}
	channel["path"], info["operation"], input["events"] = "/default/deny", "SUBSCRIBE", nil
	_, failure := app.Resolve(ctx, input)
	result["authorization"] = errors.Is(failure, denied)
	value, err := app.Resolve(ctx, map[string]any{"invalid": true})
	result["invalid"] = value == nil && err == nil
	return result, nil
}
