package protobuf

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"math"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestNativeDescriptorPreservesInt64UnknownFieldsAndOwnership(t *testing.T) {
	descriptor := (&wrapperspb.Int64Value{}).ProtoReflect().Descriptor()
	wire, err := proto.Marshal(wrapperspb.Int64(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	unknown := []byte{0x98, 0x06, 0x01}
	wire = append(wire, unknown...)
	var decoder Decoder
	message := FromDescriptor(descriptor)
	var previous proto.Message
	for range 2 {
		value, err := decoder.Deserialize(context.Background(), base64.StdEncoding.EncodeToString(wire), message, map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		native := value.(proto.Message)
		field := descriptor.Fields().ByNumber(1)
		if native.ProtoReflect().Get(field).Int() != math.MaxInt64 || !bytes.Equal(native.ProtoReflect().GetUnknown(), unknown) {
			t.Fatal("native protobuf precision or unknown fields lost")
		}
		encoded, err := protojson.Marshal(native)
		if err != nil || string(encoded) != `"9223372036854775807"` {
			t.Fatalf("official native JSON=%s, %v", encoded, err)
		}
		if native == previous {
			t.Fatal("decoder reused a mutable message")
		}
		native.ProtoReflect().Clear(field)
		native.ProtoReflect().SetUnknown(nil)
		previous = native
	}
}

func TestNativeDescriptorFailureBoundaries(t *testing.T) {
	var decoder Decoder
	for _, test := range []struct {
		name    string
		message *Message
		data    string
	}{
		{"nil descriptor", FromDescriptor(nil), ""},
		{"truncated value", FromDescriptor((&wrapperspb.Int64Value{}).ProtoReflect().Descriptor()), "CA=="},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := decoder.Deserialize(context.Background(), test.data, test.message, map[string]any{})
			var failure *kafka.DeserializationError
			if value != nil || !errors.As(err, &failure) || failure.Unwrap() == nil {
				t.Fatalf("native decode failure lost category/cause: %v, %v", value, err)
			}
		})
	}
}
