package kafkamodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	kafkaavro "github.com/rambow-cloud/powertools-lambda-go/kafka/avro"
	kafkaproto "github.com/rambow-cloud/powertools-lambda-go/kafka/protobuf"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type invocationKey struct{}

func normalize(value any) any {
	switch v := value.(type) {
	case kafka.Undefined:
		return map[string]any{"$": "undefined"}
	case []byte:
		items := make([]any, len(v))
		for i, b := range v {
			items[i] = float64(b)
		}
		return map[string]any{"$": "bytes", "data": items}
	case proto.Message:
		data, err := protojson.Marshal(v)
		if err != nil {
			panic(err)
		}
		var result any
		if err := json.Unmarshal(data, &result); err != nil {
			panic(err)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			result[key] = normalize(item)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = normalize(item)
		}
		return result
	}
	return value
}

func TestMixedReferenceEventsThroughNativeLambdaSDK(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version, AvroSchema, ProtobufSchema string
		ProtobufDescription                 json.RawMessage
		Cases                               []struct {
			Name   string
			Event  json.RawMessage
			Config map[string]struct {
				Type  kafka.SchemaType
				Parse bool
			}
			Expected any
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "2.35.0" || len(corpus.Cases) != 172 {
		t.Fatal("unexpected event-mode corpus")
	}
	wire, err := base64.StdEncoding.DecodeString(corpus.ProtobufSchema)
	if err != nil {
		t.Fatal(err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(wire, set); err != nil {
		t.Fatal(err)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := files.FindDescriptorByName("modes.Payload")
	if err != nil {
		t.Fatal(err)
	}
	var description bytes.Buffer
	if err := json.Compact(&description, corpus.ProtobufDescription); err != nil {
		t.Fatal(err)
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			for _, native := range []bool{false, true} {
				calls := []any{}
				var decoder kafkaproto.Decoder
				config := kafka.Config{}
				for name, spec := range tc.Config {
					var field *kafka.FieldConfig
					switch spec.Type {
					case kafka.JSON:
						field = &kafka.FieldConfig{Type: kafka.JSON}
					case kafka.Avro:
						field = kafkaavro.New(corpus.AvroSchema)
					case kafka.Protobuf:
						message := kafkaproto.FromDescriptor(descriptor.(protoreflect.MessageDescriptor))
						message.Description = description.Bytes()
						field = decoder.Field(message)
					default:
						t.Fatal("unknown fixture codec", spec.Type)
					}
					if spec.Parse {
						field.Parser = func(ctx context.Context, value any) (kafka.ParseResult, error) {
							if ctx.Value(invocationKey{}) != tc.Name {
								t.Error("parser lost invocation context")
							}
							calls = append(calls, map[string]any{"field": name, "value": normalize(value)})
							return kafka.ParseResult{Value: map[string]any{"parsed": value}}, nil
						}
					}
					if name == "key" {
						config.Key = field
					} else {
						config.Value = field
					}
				}
				consumer := kafka.New(config)
				handler := kafka.WrapHandler(consumer, func(ctx context.Context, event *kafka.ConsumerRecords) (map[string]any, error) {
					if ctx.Value(invocationKey{}) != tc.Name || len(event.Records) != 1 {
						t.Fatal("handler context or record count changed")
					}
					r := event.Records[0]
					reads := []any{}
					for _, name := range []string{"value", "key", "headers", "key", "value"} {
						var value any
						var err error
						switch name {
						case "key":
							value, err = r.Key(ctx)
						case "value":
							value, err = r.Value(ctx)
						case "headers":
							value, err = r.Headers(ctx)
						}
						var failure any
						if err != nil {
							var named interface{ ErrorName() string }
							if !errors.As(err, &named) {
								t.Fatal("unexpected untyped error", err)
							}
							failure = map[string]any{"name": named.ErrorName(), "message": err.Error()}
						}
						reads = append(reads, map[string]any{"field": name, "value": normalize(value), "error": failure})
					}
					return map[string]any{
						"fields": event.Fields, "reads": reads, "calls": calls,
						"originalKey": normalize(r.OriginalKey), "originalValue": normalize(r.OriginalValue), "originalHeaders": normalize(r.OriginalHeaders),
						"keySchemaMetadata": normalize(r.KeySchemaMetadata), "valueSchemaMetadata": normalize(r.ValueSchemaMetadata), "custom": r.Fields["custom"],
					}, nil
				})
				ctx := context.WithValue(context.Background(), invocationKey{}, tc.Name)
				var output []byte
				if native {
					output, err = lambda.NewHandler(handler).Invoke(ctx, tc.Event)
				} else {
					value, callErr := handler(ctx, tc.Event)
					if callErr != nil {
						t.Fatal(callErr)
					}
					output, err = json.Marshal(value)
				}
				if err != nil {
					t.Fatal(err)
				}
				var actual any
				if err := json.Unmarshal(output, &actual); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, tc.Expected) {
					want, _ := json.Marshal(tc.Expected)
					t.Fatalf("native SDK=%v\nactual: %s\nexpected: %s", native, output, want)
				}
			}
		})
	}
	t.Logf("Verified %d complete events directly and through the Go Lambda SDK", len(corpus.Cases))
}

func TestSDKEventTypeLosesRegistryMetadataAndPresence(t *testing.T) {
	// The pinned SDK type has no schema metadata and uses omitempty strings.
	// Exercise this boundary so documentation cannot promise recovery after loss.
	input := []byte(`{"eventSource":"SelfManagedKafka","records":{"orders-0":[{"value":null,"key":"","valueSchemaMetadata":{"schemaId":"17"},"headers":[]}]}}`)
	var event events.KafkaEvent
	if err := json.Unmarshal(input, &event); err != nil {
		t.Fatal(err)
	}
	output, err := kafka.New(kafka.Config{}).Deserialize(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	r := output.Records[0]
	for _, value := range []any{r.OriginalKey, r.OriginalValue, r.ValueSchemaMetadata} {
		if _, missing := value.(kafka.Undefined); !missing {
			t.Fatalf("SDK type contract changed: %#v; reassess the documented boundary", value)
		}
	}
}
