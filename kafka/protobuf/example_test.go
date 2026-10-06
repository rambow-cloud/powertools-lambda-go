package protobuf_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/kafka/protobuf"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ExampleFromDescriptor() {
	message := protobuf.FromDescriptor(wrapperspb.String("").ProtoReflect().Descriptor())
	var decoder protobuf.Decoder
	value, err := decoder.Deserialize(context.Background(), "CgVoZWxsbw==", message, map[string]any{})
	if err != nil {
		panic(err)
	}
	decoded := value.(proto.Message).ProtoReflect()
	field := decoded.Descriptor().Fields().ByName("value")
	fmt.Println(decoded.Get(field).String())
	// Output: hello
}
