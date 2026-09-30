package kms

import (
	"context"
	"fmt"
	"slices"
	"time"

	mpltypes "github.com/aws/aws-cryptographic-material-providers-library/releases/go/mpl/awscryptographymaterialproviderssmithygeneratedtypes"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/smithy-go/middleware"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
)

type clientSupplier struct {
	context context.Context
	clients ClientProvider
}

func (s clientSupplier) GetClient(input mpltypes.GetClientInput) (awskms.Client, error) {
	if err := s.context.Err(); err != nil {
		return awskms.Client{}, err
	}
	client, err := s.clients(s.context, input.Region)
	if err != nil {
		return awskms.Client{}, err
	}
	if client == nil {
		return awskms.Client{}, fmt.Errorf("KMS client provider returned nil")
	}
	options := client.Options()
	options.APIOptions = append(slices.Clone(options.APIOptions), awssdk.UserAgent("data-masking"), func(stack *middleware.Stack) error {
		return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("PowertoolsEncryptionContext", func(ctx context.Context, input middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
			return next.HandleInitialize(operationContext{Context: ctx, invocation: s.context}, input)
		}), middleware.Before)
	})
	return *awskms.New(options), nil
}

// The generated ESDK/MPL bridge drops context.Context. Restore invocation
// cancellation and values at the SDK initialization boundary, before signing,
// retries and network I/O, while retaining the SDK's own context metadata.
type operationContext struct {
	context.Context
	invocation context.Context
}

func (c operationContext) Deadline() (time.Time, bool) { return c.invocation.Deadline() }
func (c operationContext) Done() <-chan struct{}       { return c.invocation.Done() }
func (c operationContext) Err() error                  { return c.invocation.Err() }
func (c operationContext) Value(key any) any {
	if value := c.Context.Value(key); value != nil {
		return value
	}
	return c.invocation.Value(key)
}
