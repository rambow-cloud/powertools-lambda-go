package kms_test

import (
	"context"

	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
	"github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
)

// This example requires configured AWS credentials and a usable KMS key.
// It is compiled as documentation but is not executed by go test.
func ExampleNew() {
	ctx := context.Background()
	provider, err := kms.New(ctx, kms.Config{Keys: []string{"alias/example"}})
	if err != nil {
		panic(err)
	}
	masker := datamasking.New(datamasking.Config{Provider: provider})
	value, err := masker.Encrypt(ctx, map[string]any{"email": "user@example.com"},
		datamasking.TransformOptions{Fields: []string{"email"},
			Context: map[string]string{"purpose": "example"}})
	if err != nil {
		panic(err)
	}
	_ = value // Persist ciphertext; supply the same expected context on decryption.
}
