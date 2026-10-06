package avro

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	codec "github.com/hamba/avro/v2"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/kafka"
)

// New configures an Avro field. Schema parsing remains lazy, as in the reference.
// The returned configuration can be supplied as either the Kafka key or value.
func New(schema string) *kafka.FieldConfig {
	return &kafka.FieldConfig{Type: kafka.Avro, Schema: schema, Decoder: Deserialize}
}

// Deserialize decodes one datum, rejecting trailing bytes and unsafe long values.
// Schema registry metadata does not change Avro decoding in the pinned reference.
func Deserialize(ctx context.Context, data string, schema, _ any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	text, ok := schema.(string)
	if !ok {
		return nil, failure(data, fmt.Sprint(schema), errors.New("Avro schema must be a string"))
	}
	parsed, err := parseSchema(text)
	if err != nil {
		return nil, failure(data, text, err)
	}
	buffer := commons.DecodeBase64Buffer(data)
	// No byte/string datum can legitimately exceed the complete encoded input.
	// This bound avoids allocating from untrusted declared lengths before EOF.
	limit := len(buffer)
	if limit == 0 {
		limit = 1
	}
	r := codec.NewReader(nil, 0, codec.WithReaderConfig(codec.Config{MaxByteSliceSize: limit}.Freeze())).Reset(buffer)
	d := decoder{ctx: ctx, reader: r, inputSize: len(buffer)}
	value, err := d.read(parsed, 0)
	if err == nil && r.Error != nil {
		err = r.Error
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "Config.MaxByteSliceSize") {
			err = errors.New("truncated buffer")
		}
		return nil, failure(data, text, err)
	}
	r.Peek()
	if !errors.Is(r.Error, io.EOF) {
		return nil, failure(data, text, errors.New("trailing data"))
	}
	return value, nil
}

func failure(data, schema string, err error) error {
	return &kafka.DeserializationError{ConsumerError: kafka.ConsumerError{
		Message: fmt.Sprintf("Failed to deserialize Avro message: Error: %s, message: %s, schema: %s", err, data, schema), Cause: err,
	}}
}

func parseSchema(text string) (codec.Schema, error) {
	var schema any
	if err := json.Unmarshal([]byte(text), &schema); err != nil {
		// avro-js accepts primitive and named schema strings without JSON quotes.
		schema = text
	}
	stripLogical(schema)
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	// A fresh cache prevents definitions from leaking between unrelated schemas.
	return codec.ParseWithCache(string(encoded), "", &codec.SchemaCache{})
}

// avro-js ignores logical annotations unless a custom logical type is registered.
// Descend only through schema positions; defaults and custom properties are data.
func stripLogical(schema any) {
	switch s := schema.(type) {
	case []any:
		for _, child := range s {
			stripLogical(child)
		}
	case map[string]any:
		delete(s, "logicalType")
		stripLogical(s["type"])
		stripLogical(s["items"])
		stripLogical(s["values"])
		if fields, ok := s["fields"].([]any); ok {
			for _, item := range fields {
				if field, ok := item.(map[string]any); ok {
					stripLogical(field["type"])
				}
			}
		}
	}
}

type decoder struct {
	ctx       context.Context
	reader    *codec.Reader
	inputSize int
}

func (d *decoder) read(schema codec.Schema, depth int) (any, error) {
	if err := d.ctx.Err(); err != nil {
		return nil, err
	}
	if d.reader.Error != nil {
		return nil, d.reader.Error
	}
	if depth > 1024 {
		return nil, errors.New("Avro nesting exceeds 1024 levels")
	}
	r := d.reader
	if ref, ok := schema.(*codec.RefSchema); ok {
		return d.read(ref.Schema(), depth+1)
	}
	switch schema.Type() {
	case codec.Null:
		return nil, nil
	case codec.Boolean:
		var data [1]byte
		r.Read(data[:])
		return data[0] != 0, r.Error
	case codec.Int:
		return referenceNumber(r.ReadLong()), r.Error
	case codec.Long:
		n := referenceNumber(r.ReadLong())
		if r.Error != nil {
			return nil, r.Error
		}
		if n < -9007199254740990 || n > 9007199254740990 {
			return nil, errors.New("potential precision loss")
		}
		return n, nil
	case codec.Float:
		return float64(r.ReadFloat()), r.Error
	case codec.Double:
		return r.ReadDouble(), r.Error
	case codec.Bytes:
		return r.ReadBytes(), r.Error
	case codec.String:
		return commons.DecodeUTF8(r.ReadBytes()), r.Error
	case codec.Fixed:
		size := schema.(*codec.FixedSchema).Size()
		if size > d.inputSize {
			return nil, errors.New("truncated buffer")
		}
		data := make([]byte, size)
		r.Read(data)
		return data, r.Error
	case codec.Enum:
		s := schema.(*codec.EnumSchema)
		index := r.ReadLong()
		if r.Error != nil {
			return nil, r.Error
		}
		if symbol, ok := s.Symbol(int(index)); ok {
			return symbol, nil
		}
		return nil, fmt.Errorf("invalid %s enum index: %d", s.FullName(), index)
	case codec.Union:
		s := schema.(*codec.UnionSchema)
		index := r.ReadLong()
		if r.Error != nil {
			return nil, r.Error
		}
		if index < 0 || index >= int64(len(s.Types())) {
			return nil, fmt.Errorf("invalid union index: %d", index)
		}
		branch := s.Types()[index]
		value, err := d.read(branch, depth+1)
		if err != nil {
			return nil, err
		}
		if branch.Type() == codec.Null {
			return nil, nil
		}
		if ref, ok := branch.(*codec.RefSchema); ok {
			branch = ref.Schema()
		}
		name := string(branch.Type())
		if named, ok := branch.(codec.NamedSchema); ok {
			name = named.FullName()
		}
		return map[string]any{name: value}, nil
	case codec.Record:
		result := map[string]any{}
		for _, field := range schema.(*codec.RecordSchema).Fields() {
			value, err := d.read(field.Type(), depth+1)
			if err != nil {
				return nil, err
			}
			result[field.Name()] = value
		}
		return result, nil
	case codec.Array, codec.Map:
		array, object := []any{}, map[string]any{}
		var child codec.Schema
		if schema.Type() == codec.Array {
			child = schema.(*codec.ArraySchema).Items()
		} else {
			child = schema.(*codec.MapSchema).Values()
		}
		for {
			count, _ := r.ReadBlockHeader()
			if r.Error != nil {
				return nil, r.Error
			}
			if count == 0 {
				break
			}
			if count < 0 {
				return nil, errors.New("invalid block count")
			}
			for range count {
				key := ""
				if schema.Type() == codec.Map {
					key = commons.DecodeUTF8(r.ReadBytes())
				}
				value, err := d.read(child, depth+1)
				if err != nil {
					return nil, err
				}
				if schema.Type() == codec.Array {
					array = append(array, value)
				} else {
					object[key] = value
				}
			}
		}
		if schema.Type() == codec.Array {
			return array, nil
		}
		return object, nil
	}
	return nil, fmt.Errorf("unsupported Avro type: %s", schema.Type())
}

// The reference switches from bitwise to binary64 arithmetic after 28 bits,
// before undoing zigzag. Rounding can lose the sign bit near the safe boundary.
// Preserve that observable behavior instead of rounding a decoded signed int64.
func referenceNumber(value int64) float64 {
	wire := uint64(value)<<1 ^ uint64(value>>63)
	if wire < 1<<28 {
		return float64(value)
	}
	result := float64(wire & ((1 << 28) - 1))
	factor := float64(1 << 28)
	for wire >>= 28; wire != 0; wire >>= 7 {
		result += float64(wire&127) * factor
		factor *= 128
	}
	if math.Mod(result, 2) != 0 {
		return -(result + 1) / 2
	}
	return result / 2
}
