package parser_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/internal/testfixture"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func orderSchema() *parser.ObjectSchema {
	return parser.Object(
		parser.Field{Name: "id", Schema: parser.String()},
		parser.Field{Name: "amount", Schema: parser.Refine(parser.Number(), func(value any) bool { return value.(float64) >= 0 }, "amount must be non-negative")},
		parser.Field{Name: "retry", Schema: parser.Number()}.WithDefault(3),
		parser.Field{Name: "note", Schema: parser.Nullable(parser.String()), Optional: true},
	)
}

func referenceSchema(name string) parser.Schema[any] {
	order := orderSchema()
	switch name {
	case "order":
		return order
	case "json":
		return parser.JSONStringified[any](order)
	case "base64":
		return parser.Base64Encoded[any](order)
	case "text":
		return parser.String()
	case "strict":
		return order.WithUnknownFields(parser.RejectUnknown)
	case "passthrough":
		return order.WithUnknownFields(parser.PreserveUnknown)
	case "transform":
		return parser.Transform[any, any](order, func(_ context.Context, value any) (any, error) {
			value.(map[string]any)["id"] = strings.ToUpper(value.(map[string]any)["id"].(string))
			return value, nil
		})
	case "sqs":
		return schemas.SqsSchema
	case "eventbridge":
		return schemas.EventBridgeSchema
	case "dataType":
		return schemas.SqsMsgAttributeDataTypeSchema
	case "unknown":
		return parser.Unknown()
	case "null":
		return parser.Null()
	case "marshalled":
		return parser.DynamoDBMarshalled[any](order)
	case "rawMarshalled":
		return parser.DynamoDBMarshalled(parser.Unknown())
	default:
		if schema, ok := unionSchemas[name]; ok {
			return schema
		}
		if schema, ok := identitySchemas[name]; ok {
			return schema
		}
		if schema, ok := serviceSchemas[name]; ok {
			return schema
		}
		if schema, ok := httpSchemas[name]; ok {
			return schema
		}
		if schema, ok := streamSchemas[name]; ok {
			return schema
		}
		panic("unknown reference schema: " + name)
	}
}

func normalizeIssues(issues []parser.Issue) any {
	copy := parser.Prefix(issues)
	if copy == nil {
		copy = []parser.Issue{}
	}
	var visit func([]parser.Issue)
	visit = func(items []parser.Issue) {
		for i := range items {
			// JSON syntax diagnostics originate in different language runtimes.
			if strings.HasPrefix(items[i].Message, "Invalid JSON - ") {
				items[i].Message = "Invalid JSON"
			}
			for _, branch := range items[i].Errors {
				visit(branch)
			}
		}
	}
	visit(copy)
	return jsonValue(copy)
}
func jsonValue(value any) any {
	encoded, err := json.Marshal(testfixture.Normalize(value))
	if err != nil {
		panic(err)
	}
	var decoded any
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		panic(err)
	}
	return decoded
}

func TestTypeScriptReference(t *testing.T) {
	runReference(t, "testdata/typescript-v2.35.0.json", 40)
}

func TestStreamTypeScriptReference(t *testing.T) {
	runReference(t, "testdata/streams-v2.35.0.json", 106)
}

func TestHTTPTypeScriptReference(t *testing.T) { runReference(t, "testdata/http-v2.35.0.json", 458) }

func TestServiceTypeScriptReference(t *testing.T) {
	runReference(t, "testdata/services-v2.35.0.json", 270)
}

var serviceSchemas = map[string]parser.Schema[any]{
	"KafkaRecordSchema": schemas.KafkaRecordSchema, "KafkaMskEventSchema": schemas.KafkaMskEventSchema, "KafkaSelfManagedEventSchema": schemas.KafkaSelfManagedEventSchema,
	"CloudFormationCustomResourceCreateSchema": schemas.CloudFormationCustomResourceCreateSchema, "CloudFormationCustomResourceDeleteSchema": schemas.CloudFormationCustomResourceDeleteSchema, "CloudFormationCustomResourceUpdateSchema": schemas.CloudFormationCustomResourceUpdateSchema,
	"TransferFamilySchema": schemas.TransferFamilySchema, "ConnectOutboundCampaignsCustomerProfileSchema": schemas.ConnectOutboundCampaignsCustomerProfileSchema, "ConnectOutboundCampaignsSchema": schemas.ConnectOutboundCampaignsSchema,
	"SesRecordSchema": schemas.SesRecordSchema, "SesSchema": schemas.SesSchema,
	"S3Schema": schemas.S3Schema, "S3SqsEventNotificationSchema": schemas.S3SqsEventNotificationSchema, "S3EventNotificationEventBridgeSchema": schemas.S3EventNotificationEventBridgeSchema, "S3ObjectLambdaEventSchema": schemas.S3ObjectLambdaEventSchema,
}

var httpSchemas = map[string]parser.Schema[any]{
	"APIGatewayCert": schemas.APIGatewayCert, "APIGatewayRecord": schemas.APIGatewayRecord, "APIGatewayStringArray": schemas.APIGatewayStringArray, "APIGatewayHttpMethod": schemas.APIGatewayHttpMethod,
	"APIGatewayEventRequestContextSchema": schemas.APIGatewayEventRequestContextSchema, "APIGatewayProxyEventSchema": schemas.APIGatewayProxyEventSchema, "APIGatewayRequestAuthorizerEventSchema": schemas.APIGatewayRequestAuthorizerEventSchema, "APIGatewayTokenAuthorizerEventSchema": schemas.APIGatewayTokenAuthorizerEventSchema,
	"APIGatewayRequestAuthorizerV2Schema": schemas.APIGatewayRequestAuthorizerV2Schema, "APIGatewayRequestContextV2Schema": schemas.APIGatewayRequestContextV2Schema, "APIGatewayProxyEventV2Schema": schemas.APIGatewayProxyEventV2Schema, "APIGatewayRequestAuthorizerEventV2Schema": schemas.APIGatewayRequestAuthorizerEventV2Schema,
	"APIGatewayProxyWebsocketEventSchema": schemas.APIGatewayProxyWebsocketEventSchema, "AlbSchema": schemas.AlbSchema, "AlbMultiValueHeadersSchema": schemas.AlbMultiValueHeadersSchema, "LambdaFunctionUrlSchema": schemas.LambdaFunctionUrlSchema, "VpcLatticeSchema": schemas.VpcLatticeSchema, "VpcLatticeV2Schema": schemas.VpcLatticeV2Schema,
}

var streamSchemas = map[string]parser.Schema[any]{
	"SnsNotificationSchema": schemas.SnsNotificationSchema, "SnsSqsNotificationSchema": schemas.SnsSqsNotificationSchema, "SnsRecordSchema": schemas.SnsRecordSchema, "SnsSchema": schemas.SnsSchema,
	"DynamoDBStreamChangeRecordBase": schemas.DynamoDBStreamChangeRecordBase, "DynamoDBStreamToKinesisChangeRecord": schemas.DynamoDBStreamToKinesisChangeRecord, "DynamoDBStreamChangeRecord": schemas.DynamoDBStreamChangeRecord, "UserIdentity": schemas.UserIdentity, "DynamoDBStreamRecord": schemas.DynamoDBStreamRecord, "DynamoDBStreamToKinesisRecord": schemas.DynamoDBStreamToKinesisRecord, "DynamoDBStreamSchema": schemas.DynamoDBStreamSchema,
	"KinesisDataStreamRecordPayload": schemas.KinesisDataStreamRecordPayload, "KinesisDataStreamRecord": schemas.KinesisDataStreamRecord, "KinesisDataStreamSchema": schemas.KinesisDataStreamSchema, "KinesisDynamoDBStreamSchema": schemas.KinesisDynamoDBStreamSchema,
	"KinesisFirehoseRecordSchema": schemas.KinesisFirehoseRecordSchema, "KinesisFirehoseSqsRecordSchema": schemas.KinesisFirehoseSqsRecordSchema, "KinesisFirehoseSchema": schemas.KinesisFirehoseSchema, "KinesisFirehoseSqsSchema": schemas.KinesisFirehoseSqsSchema,
	"CloudWatchLogEventSchema": schemas.CloudWatchLogEventSchema, "CloudWatchLogsDecodeSchema": schemas.CloudWatchLogsDecodeSchema, "CloudWatchLogsSchema": schemas.CloudWatchLogsSchema,
}

func runReference(t *testing.T, path string, count int) {
	t.Helper()
	var fixture struct {
		Version string
		Cases   []struct {
			Name, Schema, Envelope string
			Input                  any
			InputJSON              string
			AsyncRejection         *struct{ Error, Message string }
			Safe                   bool
			Expected               struct {
				Success, Thrown bool
				Error, Message  string
				Data, Original  any
				Issues          []parser.Issue
			}
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != count {
		t.Fatalf("unexpected reference corpus: %s/%d", fixture.Version, len(fixture.Cases))
	}
	validData := map[string]any{}
	for _, item := range fixture.Cases {
		if item.Name == item.Schema+"-valid" && item.Expected.Success {
			validData[item.Schema] = item.Expected.Data
		}
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			// Correct documented Cognito inputs independently of parsed output.
			// Original fixtures and all other validation failures remain unchanged.
			if data, corrected := correctedCognitoData(item.Name, item.Schema, item.Input, validData[item.Schema]); corrected {
				if len(item.Expected.Issues) != 1 {
					t.Fatal("Cognito correction must affect exactly one fixture failure")
				}
				item.Expected.Success, item.Expected.Thrown = true, false
				item.Expected.Data, item.Expected.Original, item.Expected.Issues = data, nil, nil
			}
			if (item.Schema == "CustomEmailSenderTriggerSchema" || item.Schema == "CustomSMSSenderTriggerSchema") && item.Name == item.Schema+"-empty" {
				remaining := make([]parser.Issue, 0, len(item.Expected.Issues))
				for _, issue := range item.Expected.Issues {
					if !reflect.DeepEqual(issue.Path, []any{"response"}) {
						remaining = append(remaining, issue)
					}
				}
				item.Expected.Issues = remaining
			}
			if sources, expanded := cognitoSources[item.Schema]; expanded {
				values := make([]string, len(sources))
				for i, source := range sources {
					values[i] = "\"" + source + "\""
				}
				for i := range item.Expected.Issues {
					if reflect.DeepEqual(item.Expected.Issues[i].Path, []any{"triggerSource"}) {
						item.Expected.Issues[i].Message = "Invalid option: expected one of " + strings.Join(values, "|")
					}
				}
			}
			// AWS passes custom Lambda authorizer context through to REST handlers.
			// Restore only those input fields in otherwise successful pinned cases.
			if item.Expected.Success && (item.Schema == "APIGatewayEventRequestContextSchema" || item.Schema == "APIGatewayProxyEventSchema" || item.Schema == "APIGatewayRequestAuthorizerEventSchema") {
				input := restContext(item.Input.(map[string]any))
				if authorizer, custom := input["authorizer"].(map[string]any); custom {
					if _, cognito := authorizer["claims"]; !cognito {
						copy := make(map[string]any, len(authorizer))
						for name, value := range authorizer {
							copy[name] = value
						}
						restContext(item.Expected.Data.(map[string]any))["authorizer"] = copy
					}
				}
			}
			// Update/Delete require an existing resource ID in the AWS contract.
			// Keep the pinned fixture intact and correct only this omitted field.
			if item.Schema == "CloudFormationCustomResourceUpdateSchema" || item.Schema == "CloudFormationCustomResourceDeleteSchema" {
				if input, object := item.Input.(map[string]any); object {
					identifier, present := input["PhysicalResourceId"]
					if text, valid := identifier.(string); valid {
						if item.Expected.Success {
							item.Expected.Data.(map[string]any)["PhysicalResourceId"] = text
						}
					} else {
						received := "number" // The pinned invalid-field case uses zero.
						if !present {
							received = "undefined"
						} else if identifier == nil {
							received = "null"
						}
						item.Expected.Success, item.Expected.Thrown = false, !item.Safe
						item.Expected.Data, item.Expected.Original = nil, item.Input
						item.Expected.Issues = append(item.Expected.Issues, parser.Issue{Code: "invalid_type", Expected: "string", Message: "Invalid input: expected string, received " + received, Path: []any{"PhysicalResourceId"}})
					}
				}
			}
			schema := referenceSchema(item.Schema)
			switch item.Envelope {
			case "kafka":
				schema = parser.Any(envelopes.Kafka(schema))
			case "apigateway":
				schema = envelopes.APIGateway(schema)
			case "apigatewayv2":
				schema = envelopes.APIGatewayV2(schema)
			case "lambdaurl":
				schema = envelopes.LambdaFunctionURL(schema)
			case "lattice":
				schema = envelopes.VpcLattice(schema)
			case "latticev2":
				schema = envelopes.VpcLatticeV2(schema)
			case "sqs":
				schema = parser.Any(envelopes.SQS(schema))
			case "eventbridge":
				schema = envelopes.EventBridge(schema)
			case "sns":
				schema = parser.Any(envelopes.SNS(schema))
			case "snssqs":
				schema = parser.Any(envelopes.SNSSQS(schema))
			case "kinesis":
				schema = parser.Any(envelopes.Kinesis(schema))
			case "firehose":
				schema = parser.Any(envelopes.KinesisFirehose(schema))
			case "cloudwatch":
				schema = parser.Any(envelopes.CloudWatch(schema))
			case "dynamodb":
				schema = parser.Any(envelopes.DynamoDBStream(schema))
			}
			var success bool
			var data, original any
			var issues []parser.Issue
			input := item.Input
			if item.InputJSON != "" {
				input = json.RawMessage(item.InputJSON)
			}
			// These runtime exceptions remain operational errors in Go, not validation issues.
			operational := ""
			if item.AsyncRejection != nil {
				if item.AsyncRejection.Error != "RangeError" || item.Expected.Error != "ParseError" || item.Expected.Message != "Schema parsing supports only synchronous validation" {
					t.Fatal("unexpected reference rejection")
				}
				operational = "invalid Kafka header code point: "
			} else if item.Envelope == "kafka" && item.Safe && item.Input == nil && item.Expected.Error == "TypeError" {
				operational = "cannot read Kafka eventSource from null"
			}
			if operational != "" {
				_, err := parser.SafeParse(context.Background(), input, schema)
				var failure *parser.ParseError
				if !item.Expected.Thrown || err == nil || errors.As(err, &failure) || !strings.HasPrefix(err.Error(), operational) {
					t.Fatalf("operational error mismatch: %v", err)
				}
				return
			}
			if item.Safe {
				result, err := parser.SafeParse(context.Background(), input, schema)
				if err != nil {
					t.Fatal(err)
				}
				success, data, original = result.Success, result.Data, result.OriginalEvent
				if result.Error != nil {
					issues = result.Error.Issues
				}
			} else {
				value, err := parser.Parse(context.Background(), input, schema)
				success = err == nil
				data = value
				if err != nil {
					var failure *parser.ParseError
					if !errors.As(err, &failure) {
						t.Fatal(err)
					}
					issues = failure.Issues
				}
				if (err != nil) != item.Expected.Thrown {
					t.Fatalf("throw mismatch: %v", err)
				}
			}
			if success != item.Expected.Success {
				t.Fatalf("success=%v, want %v; issues=%+v", success, item.Expected.Success, issues)
			}
			if success {
				if !reflect.DeepEqual(jsonValue(data), item.Expected.Data) {
					t.Fatalf("data=%#v, want %#v", jsonValue(data), item.Expected.Data)
				}
			} else {
				if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(item.Expected.Issues)) {
					t.Fatalf("issues=%#v, want %#v", normalizeIssues(issues), normalizeIssues(item.Expected.Issues))
				}
				if item.Safe && !reflect.DeepEqual(jsonValue(original), item.Expected.Original) {
					t.Fatalf("original event changed: %#v", original)
				}
			}
		})
	}
}
