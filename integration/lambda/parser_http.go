package main

import (
	"context"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func parserHTTPProbe(ctx context.Context) (map[string]any, error) {
	body := `{"id":"http","amount":1}`
	restContext := map[string]any{"accountId": "account", "apiId": "api", "stage": "test", "protocol": "HTTP/1.1", "identity": map[string]any{"sourceIp": "test-invoke-source-ip"}, "requestId": "id", "requestTime": "time", "requestTimeEpoch": 1, "resourcePath": "/", "httpMethod": "POST", "path": "/"}
	rest := map[string]any{"resource": "/", "path": "/", "httpMethod": "POST", "queryStringParameters": nil, "multiValueQueryStringParameters": nil, "requestContext": restContext, "body": body, "isBase64Encoded": false}
	httpContext := map[string]any{"accountId": "account", "apiId": "api", "domainName": "example.test", "domainPrefix": "example", "http": map[string]any{"method": "POST", "path": "/", "protocol": "HTTP/1.1", "sourceIp": "2001:db8::1", "userAgent": "agent"}, "requestId": "id", "routeKey": "POST /", "stage": "test", "time": "time", "timeEpoch": 1}
	http := map[string]any{"version": "2.0", "routeKey": "POST /", "rawPath": "/", "rawQueryString": "", "headers": map[string]any{}, "requestContext": httpContext, "body": body, "isBase64Encoded": false}
	lattice := map[string]any{"method": "POST", "raw_path": "/", "body": body, "is_base64_encoded": false, "headers": map[string]any{}, "query_string_parameters": map[string]any{}}
	latticeV2 := map[string]any{"version": "2.0", "path": "/", "method": "POST", "headers": map[string]any{}, "body": body, "requestContext": map[string]any{"serviceNetworkArn": "arn:network", "serviceArn": "arn:service", "targetGroupArn": "arn:target", "region": "ap-east-1", "timeEpoch": "1", "identity": map[string]any{}}}
	payload := parser.JSONStringified(orderInputSchema)
	cases := []struct {
		name   string
		event  any
		schema parser.Schema[order]
	}{
		{"apigateway", rest, envelopes.APIGateway(payload)},
		{"apigatewayv2", http, envelopes.APIGatewayV2(payload)},
		{"lambdaurl", http, envelopes.LambdaFunctionURL(payload)},
		{"lattice", lattice, envelopes.VpcLattice(payload)},
		{"latticev2", latticeV2, envelopes.VpcLatticeV2(payload)},
	}
	result := map[string]any{}
	for _, item := range cases {
		value, err := parser.Parse(ctx, item.event, item.schema)
		if err != nil {
			return nil, err
		}
		result[item.name] = value
	}
	alb := map[string]any{"httpMethod": "CUSTOM", "path": "/", "body": body, "isBase64Encoded": false, "multiValueHeaders": map[string]any{"accept": []any{"json"}}, "multiValueQueryStringParameters": map[string]any{}, "requestContext": map[string]any{"elb": map[string]any{"targetGroupArn": "arn:target"}}}
	value, err := parser.Parse(ctx, alb, schemas.AlbMultiValueHeadersSchema)
	if err != nil {
		return nil, err
	}
	result["alb"] = value.(map[string]any)["multiValueHeaders"]
	invalid := map[string]any{"version": "2.0", "path": "/", "method": "OTHER", "headers": map[string]any{}, "body": `{"id":1,"amount":-1}`, "requestContext": latticeV2["requestContext"]}
	safe, err := parser.SafeParse(ctx, invalid, envelopes.VpcLatticeV2(payload))
	if err != nil {
		return nil, err
	}
	if safe.Error != nil {
		result["invalid_issues"] = safe.Error.Issues
	}
	return result, nil
}
