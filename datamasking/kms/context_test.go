package kms

import (
	"context"
	"testing"

	mpl "github.com/aws/aws-cryptographic-material-providers-library/releases/go/mpl/awscryptographymaterialproviderssmithygenerated"
	mpltypes "github.com/aws/aws-cryptographic-material-providers-library/releases/go/mpl/awscryptographymaterialproviderssmithygeneratedtypes"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
)

func TestClientSupplierCancellationCrossesGeneratedBridge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	supplier := clientSupplier{context: ctx, clients: func(context.Context, string) (*awskms.Client, error) {
		t.Error("cancelled supplier invoked the client factory")
		return nil, nil
	}}
	_, err := supplier.GetClient(mpltypes.GetClientInput{Region: "ap-east-1"})
	// Exercise the same generated conversion that used to panic on this path.
	converted := mpl.Error_FromDafny(mpl.Error_ToDafny(err))
	opaque, ok := converted.(mpltypes.OpaqueError)
	if !ok || opaque.ErrObject != context.Canceled {
		t.Fatalf("cancellation lost across the actual MPL bridge: %T, %v", converted, converted)
	}
}
