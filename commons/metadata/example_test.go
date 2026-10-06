package metadata_test

import (
	"context"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
)

// This example requires a Lambda environment with the metadata endpoint enabled.
func ExampleGetMetadata() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	value, err := metadata.GetMetadata(ctx)
	if err != nil {
		panic(err)
	}
	_ = value // Consume metadata without including it in application logs.
}
