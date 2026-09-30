package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	appconfigsdk "github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	ddbsdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	secretssdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	ssmsdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfig"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/appconfigagent"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

// newParameterProbe is enabled only by the isolated local Docker harness.
func newParameterProbe(tr *tracer.Tracer, credentials aws.CredentialsProvider, ddb *ddbsdk.Client) func(context.Context, bool) (map[string]any, error) {
	cfg := aws.Config{Region: "ap-east-1", BaseEndpoint: aws.String("http://capture:4318"), Credentials: credentials, RetryMaxAttempts: 1}
	ssmProvider := ssm.New(ssmsdk.NewFromConfig(cfg, func(o *ssmsdk.Options) { tr.InstrumentAWS(&o.APIOptions) }))
	secretsProvider := secrets.New(secretssdk.NewFromConfig(cfg, func(o *secretssdk.Options) { tr.InstrumentAWS(&o.APIOptions) }))
	ddbProvider, _ := dynamodb.New(ddb, dynamodb.Config{TableName: "local-settings"})
	appProvider, _ := appconfig.New(appconfigsdk.NewFromConfig(cfg, func(o *appconfigsdk.Options) { tr.InstrumentAWS(&o.APIOptions) }), appconfig.Config{Application: "local", Environment: "test"})
	agentClient := tr.HTTPClient(nil)
	return func(ctx context.Context, force bool) (map[string]any, error) {
		opts := parameters.Options{MaxAge: parameters.Age(time.Hour), Transform: parameters.JSON, ForceFetch: force}
		result := make(map[string]any)
		var err error
		if result["ssm"], err = ssmProvider.Get(ctx, "/local/flags", ssm.GetOptions{Options: opts}); err != nil {
			return nil, err
		}
		if result["secrets"], err = secretsProvider.Get(ctx, "local-secret", secrets.GetOptions{Options: opts}); err != nil {
			return nil, err
		}
		if result["dynamodb"], err = ddbProvider.Get(ctx, "flags", dynamodb.GetOptions{Options: opts}); err != nil {
			return nil, err
		}
		if result["appconfig"], err = appProvider.Get(ctx, "flags", appconfig.GetOptions{Options: opts}); err != nil {
			return nil, err
		}
		if result["agent"], err = appconfigagent.GetConfig(ctx, "flags", appconfigagent.Options{Application: "local", Environment: "test", Endpoint: "http://capture:4318", Transform: parameters.JSON, HTTPClient: agentClient}); err != nil {
			return nil, err
		}
		return result, nil
	}
}
