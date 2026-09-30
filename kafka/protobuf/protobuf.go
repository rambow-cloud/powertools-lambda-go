// Package protobuf adapts caller-supplied Protobuf message decoders to Kafka.
package protobuf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Input preserves the original buffer, prefix position and explicit-length flag.
// A negative position can occur with a malformed Confluent index in the reference.
// Custom decoders own their input validation, just as a supplied messageType does.
type Input struct {
	Buffer         []byte
	Position       int
	ExplicitLength bool
}

// Message is the Go equivalent of a supplied messageType.decode method.
// Description is optional JSON used in diagnostic messages; it defaults to {}.
// The decoder must be concurrency-safe and must not retain mutable shared results.
type Message struct {
	Decode      func(Input) (any, error)
	Description json.RawMessage
}

// FromDescriptor supports generated and runtime descriptors through the official
// Go Protobuf implementation. Results are fresh proto.Message values; protobufjs
// object defaults and JavaScript serialization are not applied to native messages.
func FromDescriptor(descriptor protoreflect.MessageDescriptor) *Message {
	return &Message{Decode: func(input Input) (any, error) {
		if descriptor == nil {
			return nil, errors.New("Protobuf message descriptor is required")
		}
		if input.Position < 0 || input.Position > len(input.Buffer) {
			return nil, &namedError{"RangeError", "invalid Protobuf reader position"}
		}
		message := dynamicpb.NewMessage(descriptor)
		if err := proto.Unmarshal(input.Buffer[input.Position:], message); err != nil {
			return nil, err
		}
		return message, nil
	}}
}

// Decoder retains the preferred Confluent index convention. The zero value starts
// with int32, like the reference. A successful fallback updates it atomically.
// No mutex is held while calling application code.
type Decoder struct{ zigzag atomic.Bool }

var shared Decoder

// New shares the process-wide index preference, as the reference module does.
func New(message *Message) *kafka.FieldConfig { return shared.Field(message) }

// Field uses this decoder's preference, allowing explicit application isolation.
func (d *Decoder) Field(message *Message) *kafka.FieldConfig {
	var schema any
	if message != nil {
		schema = message
	}
	return &kafka.FieldConfig{Type: kafka.Protobuf, Schema: schema, Decoder: d.Deserialize}
}

func (d *Decoder) Deserialize(ctx context.Context, data string, schema, metadata any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	message, ok := schema.(*Message)
	if !ok || message == nil || message.Decode == nil {
		return nil, &kafka.DeserializationError{ConsumerError: kafka.ConsumerError{Message: "Protobuf message decoder is required"}}
	}
	buffer := commons.DecodeBase64Buffer(data)
	id := any(kafka.Undefined{})
	if metadata == nil {
		return nil, failure(message, data, &namedError{"TypeError", "Cannot read properties of null (reading 'schemaId')"})
	}
	if _, absent := metadata.(kafka.Undefined); absent {
		return nil, failure(message, data, &namedError{"TypeError", "Cannot read properties of undefined (reading 'schemaId')"})
	}
	if fields, ok := metadata.(map[string]any); ok {
		if value, present := fields["schemaId"]; present {
			id = value
		}
	}
	if _, absent := id.(kafka.Undefined); absent {
		value, err := message.Decode(Input{Buffer: buffer, ExplicitLength: true})
		if err != nil {
			return nil, failure(message, data, err)
		}
		return value, nil
	}
	if id == nil {
		return nil, failure(message, data, &namedError{"TypeError", "Cannot read properties of null (reading 'length')"})
	}
	if longSchemaID(id) {
		_, position, err := uint32Prefix(buffer)
		if err != nil {
			return nil, failure(message, data, err)
		}
		value, err := message.Decode(Input{Buffer: buffer, Position: position})
		if err != nil {
			return nil, failure(message, data, err)
		}
		return value, nil
	}
	preferred := d.zigzag.Load()
	decode := func(zigzag bool) (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		index, position, err := uint32Prefix(buffer)
		if err != nil {
			return nil, err
		}
		count := int32(index)
		if zigzag {
			count = int32(index>>1) ^ -int32(index&1)
		}
		if int64(position)+int64(count) > int64(len(buffer)) {
			return nil, rangeError(position, int64(count), len(buffer))
		}
		return message.Decode(Input{Buffer: buffer, Position: position + int(count)})
	}
	value, first := decode(preferred)
	if first == nil {
		return value, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, second := decode(!preferred)
	if second == nil {
		d.zigzag.Store(!preferred)
		return value, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Both attempts failed: upstream reports the first exception.
	return nil, failure(message, data, first)
}

// Upstream tests schemaId.length > 10, including JSON objects with a length
// property. Keep its numeric string conversion in Commons. Non-JSON prototypes
// and cyclic array coercion are outside this mapping.
func longSchemaID(id any) bool {
	switch value := id.(type) {
	case string:
		length := 0
		for _, r := range value {
			length++
			if r > 0xffff {
				length++
			}
		}
		return length > 10
	case []any:
		return len(value) > 10
	case map[string]any:
		length := value["length"]
		// A one-element array stringifies to its element; other arrays cannot
		// produce a number above ten. Bound native cyclic array traversal.
		for range 1024 {
			array, ok := length.([]any)
			if !ok {
				return commons.ParseNumber(fmt.Sprint(length)) > 10
			}
			if len(array) != 1 {
				return false
			}
			length = array[0]
		}
	}
	return false
}

// uint32Prefix matches protobufjs's five-byte low-word read and ten-byte skip,
// including its reported position on truncated input. It is not a uint64 parser.
func uint32Prefix(buffer []byte) (uint32, int, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		if i < len(buffer) {
			mask := byte(127)
			if i == 4 {
				mask = 15
			}
			value |= uint32(buffer[i]&mask) << uint(7*i)
			if buffer[i] < 128 {
				return value, i + 1, nil
			}
		}
	}
	if len(buffer) < 10 {
		return 0, len(buffer), rangeError(len(buffer), 10, len(buffer))
	}
	return value, 10, nil
}

type namedError struct{ name, message string }

func (e *namedError) Error() string     { return e.message }
func (e *namedError) ErrorName() string { return e.name }
func rangeError(position int, count int64, length int) error {
	return &namedError{"RangeError", fmt.Sprintf("index out of range: %d + %d > %d", position, count, length)}
}

func failure(message *Message, data string, err error) error {
	description := message.Description
	if len(description) == 0 {
		description = json.RawMessage(`{}`)
	}
	name := "Error"
	var named interface{ ErrorName() string }
	if errors.As(err, &named) {
		name = named.ErrorName()
	}
	return &kafka.DeserializationError{ConsumerError: kafka.ConsumerError{Message: fmt.Sprintf("Failed to deserialize Protobuf message: %s: %s, message: %s, messageType: %s", name, err, data, description), Cause: err}}
}
