package protobuf

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func bytesValue(data []byte) []any {
	out := make([]any, len(data))
	for i, b := range data {
		out[i] = float64(b)
	}
	return out
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version, Schema string
		Cases           []struct {
			Name, Data, Mode       string
			Metadata, Value, Calls any
			Error                  *struct{ Name, Message string }
		}
		Native []struct {
			Mode, Data      string
			Metadata, Value any
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "2.35.0" || len(corpus.Cases) < 220 || len(corpus.Native) != 9 {
		t.Fatal("unexpected corpus")
	}
	// The reference preference persists across scenarios, including failed reads.
	var decoder Decoder
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			calls := []any{}
			message := &Message{Decode: func(input Input) (any, error) {
				calls = append(calls, map[string]any{"position": float64(input.Position), "explicitLength": input.ExplicitLength, "buffer": bytesValue(input.Buffer)})
				if tc.Mode == "reject" {
					return nil, &namedError{"RangeError", fmt.Sprintf("decoder rejected at %d", input.Position)}
				}
				if tc.Mode == "marker" && (input.Position < 0 || input.Position >= len(input.Buffer) || input.Buffer[input.Position] != 42) {
					return nil, fmt.Errorf("expected marker at %d", input.Position)
				}
				start := input.Position
				if start < 0 {
					start += len(input.Buffer)
				}
				if start < 0 {
					start = 0
				}
				if start > len(input.Buffer) {
					start = len(input.Buffer)
				}
				length := any(map[string]any{"$": "undefined"})
				if input.ExplicitLength {
					length = float64(len(input.Buffer))
				}
				return map[string]any{"position": float64(input.Position), "length": length, "bytes": bytesValue(input.Buffer[start:])}, nil
			}}
			metadata := tc.Metadata
			if tag, ok := metadata.(map[string]any); ok && tag["$"] == "undefined" {
				metadata = kafka.Undefined{}
			}
			consumer := kafka.New(kafka.Config{Value: decoder.Field(message)})
			event, err := consumer.Deserialize(context.Background(), map[string]any{"records": map[string]any{"topic": []any{map[string]any{"value": tc.Data, "valueSchemaMetadata": metadata}}}})
			if err != nil {
				t.Fatal(err)
			}
			value, err := event.Records[0].Value(context.Background())
			if tc.Error != nil {
				var failure *kafka.DeserializationError
				if !errors.As(err, &failure) || failure.ErrorName() != tc.Error.Name || failure.Error() != tc.Error.Message {
					t.Fatalf("actual: %v\nexpected: %s: %s", err, tc.Error.Name, tc.Error.Message)
				}
			} else if err != nil || !reflect.DeepEqual(value, tc.Value) {
				t.Fatalf("actual: %#v (%v)\nexpected: %#v", value, err, tc.Value)
			}
			if !reflect.DeepEqual(calls, tc.Calls) {
				t.Fatalf("calls: %#v\nexpected: %#v", calls, tc.Calls)
			}
		})
	}
	encoded, err := base64.StdEncoding.DecodeString(corpus.Schema)
	if err != nil {
		t.Fatal(err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(encoded, set); err != nil {
		t.Fatal(err)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := files.FindDescriptorByName("fixture.Payload")
	if err != nil {
		t.Fatal(err)
	}
	consumer := kafka.New(kafka.Config{Value: decoder.Field(FromDescriptor(descriptor.(protoreflect.MessageDescriptor)))})
	for i, tc := range corpus.Native {
		t.Run(fmt.Sprintf("native-%d-%s", i, tc.Mode), func(t *testing.T) {
			event, err := consumer.Deserialize(context.Background(), map[string]any{"records": map[string]any{"topic": []any{map[string]any{"value": tc.Data, "valueSchemaMetadata": tc.Metadata}}}})
			if err != nil {
				t.Fatal(err)
			}
			value, err := event.Records[0].Value(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := protojson.Marshal(value.(proto.Message))
			if err != nil {
				t.Fatal(err)
			}
			var actual any
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, tc.Value) {
				t.Fatalf("actual: %s\nexpected: %#v", encoded, tc.Value)
			}
		})
	}
	t.Logf("Verified %d prefix scenarios and %d native messages", len(corpus.Cases), len(corpus.Native))
}

func TestConcurrentFallbackAndReentrantDecoder(t *testing.T) {
	var decoder Decoder
	message := &Message{Decode: func(input Input) (any, error) {
		if input.Position >= len(input.Buffer) || input.Position < 0 || input.Buffer[input.Position] != 42 {
			return nil, errors.New("missing marker")
		}
		return input.Buffer[input.Position], nil
	}}
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			data := "AgAq"
			if i%2 == 1 {
				data = "AQAq"
			}
			value, err := decoder.Deserialize(context.Background(), data, message, map[string]any{"schemaId": "1"})
			if err != nil || value != byte(42) {
				t.Error(value, err)
			}
		})
	}
	wg.Wait()
	inner := &Message{Decode: func(input Input) (any, error) { return len(input.Buffer), nil }}
	outer := &Message{Decode: func(Input) (any, error) {
		return decoder.Deserialize(context.Background(), "Kg==", inner, map[string]any{})
	}}
	if value, err := decoder.Deserialize(context.Background(), "ACo=", outer, map[string]any{"schemaId": "1"}); value != 1 || err != nil {
		t.Fatal(value, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := decoder.Deserialize(canceled, "", outer, map[string]any{}); err != context.Canceled {
		t.Fatal(err)
	}
}
