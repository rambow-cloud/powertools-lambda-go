// Package kms implements Data Masking providers with the AWS Encryption SDK.
// This initial provider does not implement TypeScript's data-key cache.
package kms

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"slices"
	"strings"

	mpl "github.com/aws/aws-cryptographic-material-providers-library/releases/go/mpl/awscryptographymaterialproviderssmithygenerated"
	mpltypes "github.com/aws/aws-cryptographic-material-providers-library/releases/go/mpl/awscryptographymaterialproviderssmithygeneratedtypes"
	esdk "github.com/aws/aws-encryption-sdk/releases/go/encryption-sdk/awscryptographyencryptionsdksmithygenerated"
	esdktypes "github.com/aws/aws-encryption-sdk/releases/go/encryption-sdk/awscryptographyencryptionsdksmithygeneratedtypes"
	"github.com/aws/aws-sdk-go-v2/config"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

// ClientProvider supplies a regional AWS SDK client template. The provider
// clones its options to bind each operation's context without mutating it.
// Implementations must support concurrent calls.
type ClientProvider func(context.Context, string) (*awskms.Client, error)

type Config struct {
	// Keys contains the generator identifier first, followed by additional keys.
	Keys []string
	// A nil ClientProvider uses the default AWS configuration and regional clients.
	ClientProvider ClientProvider
}

// Provider uses authenticated AWS Encryption SDK messages and strict key
// commitment. It is safe for concurrent calls. Each encryption obtains fresh
// materials; cache age/capacity/message/byte limits are not implemented yet.
type Provider struct {
	keys      []string
	clients   ClientProvider
	materials *mpl.Client
	sdk       *esdk.Client
}

var _ datamasking.Provider = (*Provider)(nil)

func New(ctx context.Context, options Config) (*Provider, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(options.Keys) == 0 {
		return nil, fmt.Errorf("Noop keyring is not allowed: Set a keyId or discovery")
	}
	provider := &Provider{keys: slices.Clone(options.Keys), clients: options.ClientProvider}
	if provider.clients == nil {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		provider.clients = func(_ context.Context, region string) (*awskms.Client, error) {
			return awskms.NewFromConfig(cfg, func(options *awskms.Options) {
				if region != "" {
					options.Region = region
				}
			}), nil
		}
	}
	var err error
	provider.materials, err = mpl.NewClient(mpltypes.MaterialProvidersConfig{})
	if err != nil {
		return nil, err
	}
	policy := mpltypes.ESDKCommitmentPolicyRequireEncryptRequireDecrypt
	provider.sdk, err = esdk.NewClient(esdktypes.AwsEncryptionSdkConfig{CommitmentPolicy: &policy})
	if err != nil {
		return nil, err
	}
	// Validate keyring construction without making an AWS API request.
	if _, err := provider.keyring(ctx); err != nil {
		return nil, err
	}
	return provider, nil
}

func (p *Provider) keyring(ctx context.Context) (mpltypes.IKeyring, error) {
	return p.materials.CreateAwsKmsMultiKeyring(ctx, mpltypes.CreateAwsKmsMultiKeyringInput{
		Generator: &p.keys[0], KmsKeyIds: slices.Clone(p.keys[1:]),
		ClientSupplier: clientSupplier{context: ctx, clients: p.clients},
	})
}

func (p *Provider) Encrypt(ctx context.Context, data string, encryptionContext map[string]string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	keyring, err := p.keyring(ctx)
	if err != nil {
		return "", err
	}
	algorithm := mpltypes.ESDKAlgorithmSuiteIdAlgAes256GcmHkdfSha512CommitKeyEcdsaP384
	output, err := p.sdk.Encrypt(ctx, esdktypes.EncryptInput{
		Plaintext:         []byte(commons.DecodeUTF8([]byte(data))),
		EncryptionContext: maps.Clone(encryptionContext), Keyring: keyring,
		AlgorithmSuiteId: &algorithm,
	})
	if failure := ctx.Err(); failure != nil {
		return "", failure
	}
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(output.Ciphertext), nil
}

func (p *Provider) Decrypt(ctx context.Context, data string, encryptionContext map[string]string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	keyring, err := p.keyring(ctx)
	if err != nil {
		return "", err
	}
	output, err := p.sdk.Decrypt(ctx, esdktypes.DecryptInput{Ciphertext: commons.DecodeBase64Buffer(data), Keyring: keyring})
	if failure := ctx.Err(); failure != nil {
		return "", failure
	}
	if err != nil {
		return "", err
	}
	// The reference verifies requested entries against the authenticated output
	// after decryption. An empty expected value must not match an absent key.
	keys := slices.Sorted(maps.Keys(encryptionContext))
	commons.SortObjectKeys(keys)
	for _, key := range keys {
		value, present := output.EncryptionContext[key]
		if !present || value != encryptionContext[key] {
			return "", &datamasking.Error{Name: "DataMaskingEncryptionError", Message: fmt.Sprintf("Encryption context mismatch for key '%s'", key)}
		}
	}
	// TextDecoder strips one leading UTF-8 BOM, unlike Buffer.toString.
	return strings.TrimPrefix(commons.DecodeUTF8(output.Plaintext), "\ufeff"), nil
}
