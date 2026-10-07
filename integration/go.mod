module github.com/rambow-cloud/powertools-lambda-go/integration

go 1.27

require (
	github.com/aws/aws-lambda-go v1.55.0
	github.com/aws/aws-sdk-go-v2 v1.47.0
	github.com/aws/aws-sdk-go-v2/credentials v1.20.4
	github.com/aws/aws-sdk-go-v2/service/appconfigdata v1.31.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.68.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.51.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.44.2
	github.com/aws/aws-sdk-go-v2/service/ssm v1.78.0
	github.com/rambow-cloud/powertools-lambda-go v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/batch v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/commons/metadata v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/commons/regex v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/datamasking v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/datamasking/kms v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncevents v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/appsyncgraphql v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/bedrock v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/http v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/metrics v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/eventhandler/http/tracer v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/idempotency v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/idempotency/cache v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/jmespath v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/kafka v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/kafka/avro v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/kafka/protobuf v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/logger v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/metrics v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/parameters v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/parser v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/signer v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/tracer v1.0.0
	github.com/rambow-cloud/powertools-lambda-go/validation v1.0.0
	github.com/redis/go-redis/v9 v9.21.0
	go.opentelemetry.io/proto/otlp v1.11.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/aws/aws-cryptographic-material-providers-library/releases/go/dynamodb v0.4.0 // indirect
	github.com/aws/aws-cryptographic-material-providers-library/releases/go/kms v0.4.0 // indirect
	github.com/aws/aws-cryptographic-material-providers-library/releases/go/mpl v0.4.0 // indirect
	github.com/aws/aws-cryptographic-material-providers-library/releases/go/primitives v0.4.0 // indirect
	github.com/aws/aws-cryptographic-material-providers-library/releases/go/smithy-dafny-standard-library v0.4.0 // indirect
	github.com/aws/aws-encryption-sdk/releases/go/encryption-sdk v0.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.19 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.33.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.21.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.41.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.32 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.40 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.107.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.42.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.46.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.50.0 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dafny-lang/DafnyRuntimeGo/v4 v4.11.3 // indirect
	github.com/dlclark/regexp2/v2 v2.8.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/hamba/avro/v2 v2.31.0 // indirect
	github.com/jmespath-community/go-jmespath v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/rambow-cloud/powertools-lambda-go/commons/awssdk v1.0.0 // indirect
	github.com/rambow-cloud/powertools-lambda-go/commons/dynamodb v1.0.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws v0.71.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.46.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230314191032-db074128a8ec // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/grpc v1.83.1 // indirect
)
