package avro_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/kafka/avro"
)

func ExampleDeserialize() {
	// Avro encodes the string length as a zigzag integer before its UTF-8 bytes.
	value, err := avro.Deserialize(context.Background(), "CmhlbGxv", `"string"`, nil)
	fmt.Println(value, err)
	// Output: hello <nil>
}
