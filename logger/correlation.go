package logger

import (
	json "encoding/json/v2"
	"fmt"
)

// CorrelationSource selects a built-in event location, not a JMESPath expression.
type CorrelationSource string

const (
	APIGatewayREST          CorrelationSource = "api_gateway_rest"
	APIGatewayHTTP          CorrelationSource = "api_gateway_http"
	AppSyncAuthorizer       CorrelationSource = "appsync_authorizer"
	AppSyncResolver         CorrelationSource = "appsync_resolver"
	ApplicationLoadBalancer CorrelationSource = "application_load_balancer"
	EventBridge             CorrelationSource = "event_bridge"
	LambdaFunctionURL       CorrelationSource = "lambda_function_url"
	S3ObjectLambda          CorrelationSource = "s3_object_lambda"
	VPCLattice              CorrelationSource = "vpc_lattice"
)

// ExtractCorrelationID supports maps and typed events with JSON field tags.
// Missing values return nil. Arbitrary query expressions belong to the JMESPath utility.
func ExtractCorrelationID(source CorrelationSource, event any) (any, error) {
	var path []string
	switch source {
	case APIGatewayREST, APIGatewayHTTP, AppSyncAuthorizer, LambdaFunctionURL:
		path = []string{"requestContext", "requestId"}
	case AppSyncResolver:
		path = []string{"request", "headers", "x-amzn-trace-id"}
	case ApplicationLoadBalancer, VPCLattice:
		path = []string{"headers", "x-amzn-trace-id"}
	case EventBridge:
		path = []string{"id"}
	case S3ObjectLambda:
		path = []string{"xAmzRequestId"}
	default:
		return nil, fmt.Errorf("unknown correlation source %q", source)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, nil
		}
		value = object[key]
	}
	return value, nil
}
