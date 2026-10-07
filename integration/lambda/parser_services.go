package main

import (
	"context"
	"encoding/base64"
	jsonv1 "encoding/json"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func parserServicesProbe(ctx context.Context) (map[string]any, error) {
	result := map[string]any{}
	record := func(body string) string {
		return fmt.Sprintf(`{"topic":"orders","partition":0,"offset":1,"timestamp":1,"timestampType":"CREATE_TIME","value":%q,"headers":[]}`, base64.StdEncoding.EncodeToString([]byte(body)))
	}
	event := func(first, second string) jsonv1.RawMessage {
		return jsonv1.RawMessage(`{"eventSource":"aws:kafka","eventSourceArn":"arn:cluster","records":{"z-0":[` + record(first) + `],"a-0":[` + record(second) + `]}}`)
	}
	envelope := envelopes.Kafka(parser.JSONStringified(orderInputSchema))
	value, err := parser.Parse(ctx, event(`{"id":"z","amount":1}`, `{"id":"a","amount":2}`), envelope)
	if err != nil {
		return nil, err
	}
	result["kafka"] = value
	safe, err := parser.SafeParse(ctx, event(`{"id":0,"amount":-1}`, `{"id":"a","amount":-1}`), envelope)
	if err != nil {
		return nil, err
	}
	if safe.Error != nil {
		result["kafka_issues"] = safe.Error.Issues
	}
	cases := []struct {
		name   string
		schema parser.Schema[any]
		input  string
	}{
		{"cloudformation", schemas.CloudFormationCustomResourceUpdateSchema, `{"ServiceToken":"arn:service","ResponseURL":"https://example.test/response","StackId":"stack","RequestId":"request","LogicalResourceId":"logical","ResourceType":"Custom::Type","ResourceProperties":{"new":true},"OldResourceProperties":{"old":true},"RequestType":"Update","PhysicalResourceId":"stripped"}`},
		{"transfer", schemas.TransferFamilySchema, `{"username":"user","password":"synthetic","protocol":"SFTP","serverId":"server","sourceIp":"127.0.0.1"}`},
		{"connect", schemas.ConnectOutboundCampaignsSchema, `{"InvocationMetadata":{"CampaignContext":{"CampaignId":"id","RunId":"run","ActionId":"action","CampaignName":"name"}},"Items":{"CustomerProfiles":[]}}`},
		{"ses", schemas.SesSchema, `{"Records":[{"eventSource":"aws:ses","eventVersion":"1","ses":{"mail":{"timestamp":"2026-09-15T00:00:00Z","source":"from@example.test","messageId":"id","destination":[],"headersTruncated":false,"headers":[],"commonHeaders":{"from":[],"to":[],"returnPath":"from@example.test","messageId":"id","date":"date","subject":"subject"}},"receipt":{"timestamp":"2026-09-15T00:00:00Z","processingTimeMillis":1,"recipients":[],"spamVerdict":{"status":"PASS"},"virusVerdict":{"status":"PASS"},"spfVerdict":{"status":"PASS"},"dmarcVerdict":{"status":"PASS"},"dkimVerdict":{"status":"PASS"},"dmarcPolicy":"none","action":{"type":"Lambda","invocationType":"Event","functionArn":"arn:function"}}}}]}`},
		{"s3", schemas.S3Schema, `{"Records":[{"eventVersion":"1","eventSource":"aws:s3","awsRegion":"ap-east-1","eventTime":"2026-09-15T00:00:00Z","eventName":"ObjectCreated:Put","userIdentity":{"principalId":"id"},"requestParameters":{"sourceIPAddress":"s3.amazonaws.com"},"responseElements":{"x-amz-request-id":"id","x-amz-id-2":"id2"},"s3":{"s3SchemaVersion":"1","configurationId":"config","object":{"key":"a+b%2Fc","size":-1},"bucket":{"name":"bucket","ownerIdentity":{"principalId":"id"},"arn":"arn:bucket"}}}]}`},
		{"object_lambda", schemas.S3ObjectLambdaEventSchema, `{"xAmzRequestId":"id","getObjectContext":{"inputS3Url":"url","outputRoute":"route","outputToken":"token"},"configuration":{"accessPointArn":"arn:access","supportingAccessPointArn":"arn:support","payload":{"stripped":true}},"userRequest":{"url":"url","headers":{}},"userIdentity":{"type":"AssumedRole","accountId":"account","accessKeyId":"key","principalId":"principal","arn":"arn:user","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"principal","arn":"arn:role","accountId":"account"},"attributes":{"creationDate":"date","mfaAuthenticated":"false"}}},"protocolVersion":"1"}`},
	}
	for _, item := range cases {
		value, err := parser.Parse(ctx, jsonv1.RawMessage(item.input), item.schema)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.name, err)
		}
		result[item.name] = value
	}
	return result, nil
}
