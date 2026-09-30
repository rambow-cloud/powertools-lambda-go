// Package main is a disposable AWS integration-test handler, not a production example.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/batch"
	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/metrics"
	"github.com/rambow-cloud/powertools-lambda-go/signer"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type Event struct {
	ID           string `json:"id"`
	Mode         string `json:"mode"`
	TraceHeader  string `json:"trace_header"`
	Traceparent  string `json:"traceparent,omitempty"`
	AddTemporary bool   `json:"add_temporary"`
	QueryBody    string `json:"query_body,omitempty"`
}
type Result struct {
	ID                   string                    `json:"id"`
	Instance             string                    `json:"instance"`
	Invocation           uint64                    `json:"invocation"`
	RequestID            string                    `json:"request_id"`
	TraceID              string                    `json:"trace_id"`
	Sampled              bool                      `json:"sampled"`
	RuntimeHeaderPresent bool                      `json:"runtime_header_present"`
	Parameters           map[string]any            `json:"parameters,omitempty"`
	Metadata             map[string]any            `json:"metadata,omitempty"`
	Query                any                       `json:"query,omitempty"`
	Batch                map[string]batch.Response `json:"batch,omitempty"`
	Idempotency          *idempotencyResult        `json:"idempotency,omitempty"`
	Cache                *cacheResult              `json:"cache,omitempty"`
	Parser               *parserResult             `json:"parser,omitempty"`
	Validation           map[string]any            `json:"validation,omitempty"`
	HTTP                 map[string]any            `json:"http,omitempty"`
	MetricsStores        map[string]any            `json:"metrics_stores,omitempty"`
	AppSyncEvents        map[string]any            `json:"appsync_events,omitempty"`
	AppSyncGraphQL       map[string]any            `json:"appsync_graphql,omitempty"`
	Bedrock              map[string]any            `json:"bedrock,omitempty"`
	Kafka                map[string]any            `json:"kafka,omitempty"`
	DataMasking          map[string]any            `json:"datamasking,omitempty"`
}

func main() {
	service := "powertools-integration-" + os.Getenv("TRACE_BACKEND")
	l := logger.New(logger.WithServiceName(service), logger.WithBuffer(logger.BufferOptions{}), logger.WithErrorHandler(func(err error) { log.Printf("LOGGER_FAILURE: %v", err) }))
	options := []tracer.Option{tracer.WithServiceName(service), tracer.WithErrorHandler(func(err error) { log.Printf("TRACER_FAILURE: %v", err) })}
	if os.Getenv("TRACE_BACKEND") == "xray" {
		log.Fatal("The legacy X-Ray SDK fixture is retired. Set TRACE_BACKEND=otel and use the collector's awsxray exporter.")
	}
	tr, err := tracer.New(options...)
	if err != nil {
		log.Fatal(err)
	}
	m, err := metrics.New(metrics.WithNamespace("PowertoolsIntegration"), metrics.WithServiceName(service), metrics.WithErrorHandler(func(err error) { log.Printf("METRICS_FAILURE: %v", err) }))
	if err != nil {
		log.Fatal(err)
	}
	// The fixture uses only Lambda-provided environment credentials; no credentials are logged.
	credentials := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: os.Getenv("AWS_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"), SessionToken: os.Getenv("AWS_SESSION_TOKEN")}, nil
	})
	ddbOptions := dynamodb.Options{Region: os.Getenv("AWS_REGION"), Credentials: credentials}
	fixtureURL := "https://" + os.Getenv("TEST_BUCKET") + ".s3." + os.Getenv("AWS_REGION") + ".amazonaws.com/fixture.txt"
	if os.Getenv("LOCAL_TEST") == "true" {
		// Docker fixtures use an internal-only network and synthetic credentials.
		ddbOptions.BaseEndpoint = aws.String("http://capture:4318")
		fixtureURL = "http://capture:4318/fixture.txt"
	}
	ddb := dynamodb.New(ddbOptions, func(o *dynamodb.Options) { tr.InstrumentAWS(&o.APIOptions) })
	client := tr.HTTPClient(&http.Client{Timeout: 5 * time.Second})
	requestSigner, err := signer.New(signer.Config{Service: "s3", Credentials: credentials, DisableURIPathEscaping: true})
	if err != nil {
		log.Fatal(err)
	}
	signedClient := signer.HTTPClient(requestSigner, client)
	query := jmespath.MustCompile("powertools_json(query_body).items[?enabled].id", jmespath.WithPowertoolsFunctions())
	var parameterProbe func(context.Context, bool) (map[string]any, error)
	var metadataClient *metadata.Client
	var batchProbe func(context.Context) (map[string]batch.Response, error)
	var idempotencyProbe func(context.Context, string) (idempotencyResult, error)
	var kafkaProbe func(context.Context, string) (map[string]any, error)
	var cacheProbe func(context.Context, string) (cacheResult, error)
	var httpProbe func(context.Context) (map[string]any, error)
	if os.Getenv("LOCAL_TEST") == "true" {
		parameterProbe = newParameterProbe(tr, credentials, ddb)
		batchProbe = newBatchProbe(tr, l)
		idempotencyProbe = newIdempotencyProbe(tr, l, credentials)
		kafkaProbe = newKafkaProbe(tr, l, credentials)
		cacheProbe = newCacheProbe()
		httpProbe = newHTTPProbe(l, tr, m)
		metadataClient = metadata.New(metadata.Config{Endpoint: "http://capture:4318", Token: "local-metadata-token", HTTPClient: client})
	}
	identity := make([]byte, 8)
	if _, err = rand.Read(identity); err != nil {
		log.Fatal(err)
	}
	instance := hex.EncodeToString(identity)
	var count atomic.Uint64
	handler := func(ctx context.Context, event Event) (Result, error) {
		requestMetrics := m.WithContext(ctx)
		if err := requestMetrics.AddMetadata("test_id", event.ID); err != nil {
			return Result{}, err
		}
		if err := requestMetrics.AddMetric("Invocations", metrics.Count, 1); err != nil {
			return Result{}, err
		}
		bound := l.WithContext(ctx)
		bound.AppendKeys(logger.Fields{"test_id": event.ID, "instance": instance})
		if event.AddTemporary {
			bound.AppendKeys(logger.Fields{"temporary_marker": "first-only"})
		}
		current := count.Add(1)
		_ = bound.Info("integration start", logger.Fields{"invocation": current})
		_ = bound.Debug("buffered diagnostic")
		lc, _ := lambdacontext.FromContext(ctx)
		nativeHeader, _ := ctx.Value(runtimeHeaderKey{}).(bool)
		result := Result{ID: event.ID, Instance: instance, Invocation: current, RequestID: lc.AwsRequestID, TraceID: tr.TraceID(ctx), Sampled: tr.IsTraceSampled(ctx), RuntimeHeaderPresent: nativeHeader}
		if event.QueryBody != "" {
			var queryErr error
			result.Query, queryErr = query.Search(event)
			if queryErr != nil {
				return result, queryErr
			}
		}
		_, err := tracer.Capture(ctx, tr, "business", func(ctx context.Context) (string, error) {
			if err := tr.PutAnnotation(ctx, "TestID", event.ID); err != nil {
				return "", err
			}
			if err := tr.PutMetadata(ctx, "fixture", map[string]any{"test_id": event.ID}); err != nil {
				return "", err
			}
			if event.Mode == "error" {
				return "", errors.New("intentional integration error")
			}
			if event.Mode == "panic" {
				panic("intentional integration panic")
			}
			if parameterProbe != nil {
				var err error
				parsed, err := parserProbe(ctx)
				if err != nil {
					return "", err
				}
				result.Parser = &parsed
				result.Validation, err = validationProbe(ctx)
				if err != nil {
					return "", err
				}
				idempotent, err := idempotencyProbe(ctx, event.ID)
				if err != nil {
					return "", err
				}
				result.Idempotency = &idempotent
				cached, err := cacheProbe(ctx, event.ID)
				if err != nil {
					return "", err
				}
				result.Cache = &cached
				result.Batch, err = batchProbe(ctx)
				if err != nil {
					return "", err
				}
				result.HTTP, err = httpProbe(ctx)
				if err != nil {
					return "", err
				}
				result.MetricsStores, err = metricsStoreProbe(ctx, m)
				if err != nil {
					return "", err
				}
				result.AppSyncEvents, err = appSyncEventsProbe(ctx, l, tr)
				if err != nil {
					return "", err
				}
				result.AppSyncGraphQL, err = graphQLProbe(ctx, l, tr)
				if err != nil {
					return "", err
				}
				result.Bedrock, err = bedrockProbe(ctx, l, tr)
				if err != nil {
					return "", err
				}
				result.Kafka, err = kafkaProbe(ctx, event.ID)
				if err != nil {
					return "", err
				}
				result.DataMasking, err = dataMaskingProbe(ctx)
				if err != nil {
					return "", err
				}
				result.Parameters, err = parameterProbe(ctx, event.Mode == "unsampled")
				if err != nil {
					return "", err
				}
				if event.Mode == "unsampled" {
					metadataClient.ClearCache()
				}
				result.Metadata, err = metadataClient.Get(ctx)
				if err != nil {
					return "", err
				}
			}
			_, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(os.Getenv("TEST_TABLE")), Key: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: event.ID}}})
			if err != nil {
				return "", err
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, fixtureURL, nil)
			if err != nil {
				return "", err
			}
			response, err := signedClient.Do(request)
			if err != nil {
				return "", err
			}
			defer response.Body.Close()
			body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
			if err != nil {
				return "", err
			}
			if response.StatusCode != 200 || strings.TrimSpace(string(body)) != "powertools-integration" {
				return "", fmt.Errorf("unexpected S3 fixture response: %d", response.StatusCode)
			}
			return "ok", nil
		})
		return result, err
	}
	lambda.Start(tracer.WrapHandler(tr, metrics.WrapHandler(m, logger.WrapHandler(l, handler, logger.HandlerOptions{FlushBufferOnError: true, CorrelationExtractor: jmespath.MustCompile("join(':', ['query', id])")}), metrics.HandlerOptions{CaptureColdStart: true}), tracer.HandlerOptions{Name: "integration", ExtractContext: func(ctx context.Context, value any) context.Context {
		header, _ := ctx.Value("x-amzn-trace-id").(string)
		ctx = context.WithValue(ctx, runtimeHeaderKey{}, header != "")
		if parent := value.(Event).Traceparent; parent != "" {
			headers := http.Header{}
			headers.Set("traceparent", parent)
			ctx = context.WithValue(ctx, "x-amzn-trace-id", "")
			return tracer.ExtractHTTPContext(ctx, headers)
		}
		// Deterministic test parents allow explicit sampled/unsampled acceptance checks.
		// An empty fixture header preserves the actual Lambda runtime parent.
		if event := value.(Event); event.TraceHeader != "" {
			ctx = context.WithValue(ctx, "x-amzn-trace-id", event.TraceHeader)
		}
		return ctx
	}}))
}

type runtimeHeaderKey struct{}
