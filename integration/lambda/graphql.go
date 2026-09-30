package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

func graphQLProbe(ctx context.Context, log *logger.Logger, tr *tracer.Tracer) (map[string]any, error) {
	event := func(field string, fail bool) map[string]any {
		return map[string]any{"arguments": map[string]any{"fail": fail}, "identity": map[string]any{"resolverContext": map[string]any{}}, "source": nil, "request": map[string]any{"headers": map[string]any{}, "domainName": nil}, "prev": nil, "info": map[string]any{"fieldName": field, "parentTypeName": "Query", "variables": map[string]any{}, "selectionSetList": []any{"id"}, "selectionSetGraphQL": "{ id }"}, "stash": map[string]any{}}
	}
	input := event("order", false)
	if _, err := parser.Parse(ctx, input, schemas.AppSyncResolverSchema); err != nil {
		return nil, err
	}
	app := appsyncgraphql.New(appsyncgraphql.Options{Diagnostic: func(context.Context, string, string, error) {}})
	app.OnQuery("order", func(ctx context.Context, args, raw any) (any, error) {
		invocation, _ := lambdacontext.FromContext(ctx)
		return map[string]any{"request_id": invocation.AwsRequestID, "trace_id": tr.TraceID(ctx), "correlation_id": log.WithContext(ctx).GetCorrelationID()}, nil
	})
	result := map[string]any{"parsed": true}
	value, err := app.Resolve(ctx, input)
	if err != nil {
		return nil, err
	}
	result["single"] = value
	app.OnBatchQuery("aggregate", func(context.Context, any, any) (any, error) { return []string{"short"}, nil })
	value, err = app.Resolve(ctx, []any{event("aggregate", false), event("different", false)})
	if err != nil {
		return nil, err
	}
	result["aggregate"] = value
	count := 0
	handler := func(_ context.Context, args, raw any) (any, error) {
		count++
		if args.(map[string]any)["fail"] == true {
			return nil, errors.New("failed")
		}
		return raw.(map[string]any)["info"].(map[string]any)["fieldName"], nil
	}
	app.OnBatchQuery("individual", handler, appsyncgraphql.BatchOptions{Individual: true})
	value, err = app.Resolve(ctx, []any{event("individual", false), event("second", true), event("third", false)})
	if err != nil {
		return nil, err
	}
	result["individual"], result["individual_calls"] = value, count
	count = 0
	app.OnBatchQuery("abort", handler, appsyncgraphql.BatchOptions{Individual: true, ThrowOnError: true})
	value, err = app.Resolve(ctx, []any{event("abort", false), event("second", true), event("third", false)})
	if err != nil {
		return nil, err
	}
	result["abort"], result["abort_calls"] = value, count
	router := appsyncgraphql.NewRouter(appsyncgraphql.Options{Diagnostic: func(context.Context, string, string, error) {}})
	router.OnException([]string{"Error"}, func(_ context.Context, err error) (any, error) { return map[string]any{"handled": err.Error()}, nil })
	app.IncludeRouter(router)
	app.OnQuery("failure", handler)
	value, err = app.Resolve(ctx, event("failure", true))
	if err != nil {
		return nil, err
	}
	result["exception"] = value
	_, err = app.Resolve(ctx, event("missing", false))
	var missing *appsyncgraphql.ResolverNotFoundException
	result["missing"] = errors.As(err, &missing)
	value, err = app.Resolve(ctx, nil)
	result["invalid"] = value == nil && err == nil
	date, err := appsyncgraphql.AWSDateTime(time.UnixMilli(0), 5.5)
	if err != nil {
		return nil, err
	}
	result["datetime"] = date
	return result, nil
}
