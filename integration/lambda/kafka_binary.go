package main

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	kafkaavro "github.com/rambow-cloud/powertools-lambda-go/kafka/avro"
	kafkaproto "github.com/rambow-cloud/powertools-lambda-go/kafka/protobuf"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func kafkaBinaryProbe(ctx context.Context) (map[string]any, error) {
	read := func(config *kafka.FieldConfig, payload []byte, metadata any) (any, error) {
		consumer := kafka.New(kafka.Config{Value: config})
		event, err := consumer.Deserialize(ctx, map[string]any{"records": map[string]any{"topic": []any{map[string]any{"value": base64.StdEncoding.EncodeToString(payload), "valueSchemaMetadata": metadata}}}})
		if err != nil {
			return nil, err
		}
		return event.Records[0].Value(ctx)
	}
	result := map[string]any{}
	avroSchema := `{"type":"record","name":"Order","fields":[{"name":"id","type":"string"},{"name":"count","type":"long"},{"name":"tag","type":["null","string"]},{"name":"raw","type":"bytes"}]}`
	value, err := read(kafkaavro.New(avroSchema), []byte{10, 'o', 'r', 'd', 'e', 'r', 14, 2, 6, 'y', 'e', 's', 4, 0, 255}, map[string]any{"dataFormat": "AVRO", "schemaId": "17"})
	if err != nil {
		return nil, err
	}
	result["avro"] = value
	_, err = read(kafkaavro.New("long"), []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 1}, nil)
	var decodeError *kafka.DeserializationError
	result["avro_precision_rejected"] = errors.As(err, &decodeError)
	descriptor := (&wrapperspb.StringValue{}).ProtoReflect().Descriptor()
	message := kafkaproto.FromDescriptor(descriptor)
	var decoder kafkaproto.Decoder
	wire, err := proto.Marshal(wrapperspb.String("order"))
	if err != nil {
		return nil, err
	}
	results := map[string]any{}
	for _, test := range []struct {
		name     string
		prefix   []byte
		metadata any
	}{
		{"plain", nil, map[string]any{}},
		{"glue", []byte{0}, map[string]any{"schemaId": "00000000-0000-0000-0000-000000000000"}},
		{"confluent", []byte{0}, map[string]any{"schemaId": "17"}},
		{"zigzag", []byte{2, 0}, map[string]any{"schemaId": "17"}},
		{"int32", []byte{1, 0}, map[string]any{"schemaId": "17"}},
		{"object_length", []byte{1}, map[string]any{"schemaId": map[string]any{"length": "0xb"}}},
	} {
		payload := append(append([]byte{}, test.prefix...), wire...)
		value, err := read(decoder.Field(message), payload, test.metadata)
		if err != nil {
			return nil, err
		}
		results[test.name] = value.(proto.Message).ProtoReflect().Get(descriptor.Fields().ByName("value")).String()
	}
	result["protobuf"] = results
	_, err = read(decoder.Field(message), wire, nil)
	result["protobuf_metadata_rejected"] = errors.As(err, &decodeError)
	return result, nil
}
