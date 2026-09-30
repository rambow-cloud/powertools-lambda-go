// Package kafka provides lazy deserialization of AWS Lambda Kafka events.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type SchemaType string

const (
	JSON     SchemaType = "json"
	Avro     SchemaType = "avro"
	Protobuf SchemaType = "protobuf"
)

// Undefined distinguishes a missing field from a null value or tombstone.
type Undefined struct{}

// Decoder is the optional binary codec boundary. Metadata is passed unchanged.
// Implementations must be safe for concurrent invocations and return fresh values.
type Decoder func(context.Context, string, any, any) (any, error)

// ParseResult follows Standard Schema: a non-nil Issues value is a validation
// failure, including an empty slice. Errors returned by Parser propagate unchanged.
type ParseResult struct {
	Value  any
	Issues []any
}

// Parser synchronously validates or transforms a decoded field.
type Parser func(context.Context, any) (ParseResult, error)

type FieldConfig struct {
	Type    SchemaType
	Schema  any
	Decoder Decoder
	Parser  Parser
}

type Config struct {
	Key, Value *FieldConfig
	// Diagnostic reports JSON parse failures before returning the decoded text.
	// The default writes to stderr. Callbacks must be concurrency-safe.
	Diagnostic func(context.Context, string, error)
}

// Consumer has immutable configuration and no per-invocation mutable state.
type Consumer struct{ config Config }

func New(config Config) *Consumer {
	if config.Key != nil {
		field := *config.Key
		config.Key = &field
	}
	if config.Value != nil {
		field := *config.Value
		config.Value = &field
	}
	if config.Diagnostic == nil {
		config.Diagnostic = func(_ context.Context, message string, err error) { fmt.Fprintln(os.Stderr, message, err) }
	}
	return &Consumer{config: config}
}

// ConsumerRecords keeps event metadata separate from the flattened records.
// Fields is a shallow copy; nested input values remain application-owned.
type ConsumerRecords struct {
	Fields  map[string]any
	Records []*Record
}

// Record exposes original fields and lazy reads. Reads are deliberately not cached:
// repeated access decodes and validates again, matching the reference getters.
type Record struct {
	Fields                                          map[string]any
	OriginalKey, OriginalValue, OriginalHeaders     any
	KeySchemaMetadata, ValueSchemaMetadata          any
	key, value, headers, keyMetadata, valueMetadata any
	consumer                                        *Consumer
}

func (r *Record) Key(ctx context.Context) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if value, ok := r.key.(string); ok && value == "" {
		return Undefined{}, nil
	}
	return r.consumer.decode(ctx, r.key, r.keyMetadata, r.consumer.config.Key)
}

func (r *Record) Value(ctx context.Context) (any, error) {
	return r.consumer.decode(ctx, r.value, r.valueMetadata, r.consumer.config.Value)
}

// Deserialize validates the event and prepares records without reading any payload.
// RawMessage preserves topic insertion order; Go maps have deterministic sorted keys.
// Like the reference guard, only the records object and its array values are required.
func (c *Consumer) Deserialize(ctx context.Context, input any) (*ConsumerRecords, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	event, keys, err := eventObject(input)
	if err != nil {
		return nil, err
	}
	topics, ok := event["records"].(map[string]any)
	if !ok {
		return nil, invalidEvent()
	}
	for _, records := range topics {
		if _, ok := records.([]any); !ok {
			return nil, invalidEvent()
		}
	}
	result := &ConsumerRecords{Fields: make(map[string]any, len(event)), Records: []*Record{}}
	for key, value := range event {
		if key != "records" {
			result.Fields[key] = value
		}
	}
	for _, key := range keys {
		for _, item := range topics[key].([]any) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if item == nil {
				return nil, &TypeError{Message: "Cannot destructure property 'key' of 'record' as it is null."}
			}
			fields := properties(item)
			// Codec selection happens before the handler, even for absent fields.
			for _, config := range []*FieldConfig{c.config.Key, c.config.Value} {
				if err := selectDecoder(config); err != nil {
					return nil, err
				}
			}
			get := func(key string) any {
				if v, ok := fields[key]; ok {
					return v
				}
				return Undefined{}
			}
			r := &Record{consumer: c, key: get("key"), value: get("value"), headers: get("headers"), keyMetadata: get("keySchemaMetadata"), valueMetadata: get("valueSchemaMetadata"), Fields: map[string]any{}}
			r.OriginalKey, r.OriginalValue, r.OriginalHeaders = r.key, r.value, r.headers
			r.KeySchemaMetadata, r.ValueSchemaMetadata = r.keyMetadata, r.valueMetadata
			for name, value := range fields {
				switch name {
				case "key", "value", "headers", "keySchemaMetadata", "valueSchemaMetadata", "originalKey", "originalValue", "originalHeaders":
				default:
					r.Fields[name] = value
				}
			}
			result.Records = append(result.Records, r)
		}
	}
	return result, nil
}

func invalidEvent() error {
	return &ConsumerError{Message: "Event is not a valid MSKEvent. Expected an object with a \"records\" property."}
}

func selectDecoder(config *FieldConfig) error {
	if config == nil {
		return nil
	}
	switch config.Type {
	case "", JSON:
		return nil
	case Avro, Protobuf:
		if config.Decoder == nil {
			return &DeserializationError{ConsumerError{Message: fmt.Sprintf("A %s decoder adapter is required", config.Type)}}
		}
		return nil
	default:
		return &DeserializationError{ConsumerError{Message: fmt.Sprintf("Unsupported deserialization type: %s. Supported types are: json, avro, protobuf.", config.Type)}}
	}
}

// WrapHandler preserves context, returned values, errors and panics. It reads no
// record fields automatically; handlers decide whether to inspect a failing field.
func WrapHandler[R any](consumer *Consumer, handler func(context.Context, *ConsumerRecords) (R, error)) func(context.Context, json.RawMessage) (R, error) {
	return func(ctx context.Context, event json.RawMessage) (R, error) {
		var zero R
		records, err := consumer.Deserialize(ctx, event)
		if err != nil {
			return zero, err
		}
		return handler(ctx, records)
	}
}
