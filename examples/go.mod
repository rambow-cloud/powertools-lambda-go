module github.com/rambow-cloud/powertools-lambda-go/examples

go 1.27.3

require (
	github.com/aws/aws-lambda-go v1.55.1
	github.com/aws/aws-sdk-go-v2 v1.47.5
	github.com/aws/aws-sdk-go-v2/config v1.33.13
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.72.3
	github.com/rambow-cloud/powertools-lambda-go/batch v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/http v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/idempotency v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/jmespath v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/kafka v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/logger v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/metrics v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/parser v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/signer v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/tracer v1.3.1
	github.com/rambow-cloud/powertools-lambda-go/validation v1.3.1
	go.opentelemetry.io/otel/sdk v1.47.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.25 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.13 // indirect
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.21.17 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.46.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.6 // indirect
	github.com/aws/smithy-go v1.28.7 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dlclark/regexp2/v2 v2.12.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.33.0 // indirect
	github.com/jmespath-community/go-jmespath v1.1.1 // indirect
	github.com/rambow-cloud/powertools-lambda-go v1.3.1 // indirect
	github.com/rambow-cloud/powertools-lambda-go/commons/awssdk v1.3.1 // indirect
	github.com/rambow-cloud/powertools-lambda-go/commons/regex v1.3.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws v0.72.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.47.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
