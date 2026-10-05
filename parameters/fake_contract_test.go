package parameters_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbsdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	secretssdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	ssmsdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

// These fakes return SDK values directly; HTTP and SDK decoding are tested separately.
// Embedded unused methods deliberately fail if a single-value read changes operation.
type readSSM struct {
	ssm.Client
	get func(context.Context, *ssmsdk.GetParameterInput) (*ssmsdk.GetParameterOutput, error)
}

func (f readSSM) GetParameter(ctx context.Context, in *ssmsdk.GetParameterInput, _ ...func(*ssmsdk.Options)) (*ssmsdk.GetParameterOutput, error) {
	return f.get(ctx, in)
}

type readSecret func(context.Context, *secretssdk.GetSecretValueInput) (*secretssdk.GetSecretValueOutput, error)

func (f readSecret) GetSecretValue(ctx context.Context, in *secretssdk.GetSecretValueInput, _ ...func(*secretssdk.Options)) (*secretssdk.GetSecretValueOutput, error) {
	return f(ctx, in)
}

type readDynamoDB struct {
	dynamodb.Client
	get func(context.Context, *ddbsdk.GetItemInput) (*ddbsdk.GetItemOutput, error)
}

func (f readDynamoDB) GetItem(ctx context.Context, in *ddbsdk.GetItemInput, _ ...func(*ddbsdk.Options)) (*ddbsdk.GetItemOutput, error) {
	return f.get(ctx, in)
}

func TestFakeServiceFailureRecoveryAndCancellation(t *testing.T) {
	for _, service := range []string{"ssm", "secrets", "dynamodb"} {
		t.Run(service, func(t *testing.T) {
			calls := 0
			var failure error
			var cancelDuringFetch context.CancelFunc
			before := func(ctx context.Context) error {
				calls++
				if ctx.Value("fixture") != "invocation" {
					t.Error("invocation context was lost")
				}
				if cancelDuringFetch != nil {
					cancelDuringFetch()
				}
				return failure
			}
			opts := parameters.Options{MaxAge: parameters.Age(time.Hour)}
			var get func(context.Context) (any, error)
			switch service {
			case "ssm":
				failure = &ssmtypes.InternalServerError{Message: aws.String("synthetic failure")}
				p := ssm.New(readSSM{get: func(ctx context.Context, in *ssmsdk.GetParameterInput) (*ssmsdk.GetParameterOutput, error) {
					if aws.ToString(in.Name) != "config" || !aws.ToBool(in.WithDecryption) {
						t.Errorf("unexpected SSM input: %+v", in)
					}
					if err := before(ctx); err != nil {
						return nil, err
					}
					return &ssmsdk.GetParameterOutput{Parameter: &ssmtypes.Parameter{Value: aws.String("value")}}, nil
				}})
				get = func(ctx context.Context) (any, error) {
					return p.Get(ctx, "config", ssm.GetOptions{Options: opts, Decrypt: aws.Bool(true)})
				}
			case "secrets":
				failure = &secretstypes.ResourceNotFoundException{Message: aws.String("synthetic missing secret")}
				p := secrets.New(readSecret(func(ctx context.Context, in *secretssdk.GetSecretValueInput) (*secretssdk.GetSecretValueOutput, error) {
					if aws.ToString(in.SecretId) != "config" || aws.ToString(in.VersionStage) != "AWSPREVIOUS" {
						t.Errorf("unexpected Secrets input: %+v", in)
					}
					if err := before(ctx); err != nil {
						return nil, err
					}
					return &secretssdk.GetSecretValueOutput{SecretString: aws.String("value")}, nil
				}))
				get = func(ctx context.Context) (any, error) {
					return p.Get(ctx, "config", secrets.GetOptions{Options: opts, SDKOptions: &secretssdk.GetSecretValueInput{VersionStage: aws.String("AWSPREVIOUS")}})
				}
			case "dynamodb":
				failure = &ddbtypes.ResourceNotFoundException{Message: aws.String("synthetic missing table")}
				p, err := dynamodb.New(readDynamoDB{get: func(ctx context.Context, in *ddbsdk.GetItemInput) (*ddbsdk.GetItemOutput, error) {
					key, ok := in.Key["id"].(*ddbtypes.AttributeValueMemberS)
					if !ok || key.Value != "config" || aws.ToString(in.TableName) != "settings" || !aws.ToBool(in.ConsistentRead) || aws.ToString(in.ProjectionExpression) != "#value" {
						t.Errorf("unexpected DynamoDB input: %+v", in)
					}
					if err := before(ctx); err != nil {
						return nil, err
					}
					return &ddbsdk.GetItemOutput{Item: map[string]ddbtypes.AttributeValue{"value": &ddbtypes.AttributeValueMemberS{Value: "value"}}}, nil
				}}, dynamodb.Config{TableName: "settings"})
				if err != nil {
					t.Fatal(err)
				}
				get = func(ctx context.Context) (any, error) {
					return p.Get(ctx, "config", dynamodb.GetOptions{Options: opts, SDKOptions: &ddbsdk.GetItemInput{ConsistentRead: aws.Bool(true)}})
				}
			}
			ctx := context.WithValue(context.Background(), "fixture", "invocation")
			for range 2 {
				_, err := get(ctx)
				var wrapped *parameters.GetParameterError
				if !errors.As(err, &wrapped) || !errors.Is(err, failure) || wrapped.Name != "config" {
					t.Fatalf("lost typed error chain: %v", err)
				}
			}
			if calls != 2 {
				t.Fatalf("failed fetch was cached: %d", calls)
			}
			failure = nil
			cancelled, cancel := context.WithCancel(ctx)
			cancelDuringFetch = cancel
			if _, err := get(cancelled); !errors.Is(err, context.Canceled) {
				t.Fatalf("late cancellation: %v", err)
			}
			cancelDuringFetch = nil
			for range 2 {
				value, err := get(ctx)
				if err != nil || value != "value" {
					t.Fatalf("recovery: %v/%v", value, err)
				}
			}
			if calls != 4 {
				t.Fatalf("cancelled result cached or warm result missed: %d", calls)
			}
			if _, err := get(cancelled); !errors.Is(err, context.Canceled) || calls != 4 {
				t.Fatalf("cancelled warm read: %v calls=%d", err, calls)
			}
		})
	}
}

func TestFakeTypedMissingParameterPolicy(t *testing.T) {
	for _, throw := range []bool{false, true} {
		t.Run(fmt.Sprint(throw), func(t *testing.T) {
			missing := &ssmtypes.ParameterNotFound{Message: aws.String("synthetic missing parameter")}
			calls := 0
			p := ssm.New(readSSM{get: func(context.Context, *ssmsdk.GetParameterInput) (*ssmsdk.GetParameterOutput, error) {
				calls++
				return nil, missing
			}})
			for range 2 {
				_, err := p.Get(context.Background(), "missing", ssm.GetOptions{Options: parameters.Options{ThrowOnMissing: throw}})
				var normalized *parameters.ParameterNotFoundError
				if throw && !errors.As(err, &normalized) || !throw && !errors.Is(err, missing) {
					t.Fatalf("missing policy: %v", err)
				}
			}
			if calls != 2 {
				t.Fatal("missing response cached")
			}
		})
	}
}
