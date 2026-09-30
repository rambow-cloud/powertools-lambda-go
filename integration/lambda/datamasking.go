package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/commons/regex"
	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

// maskingFixtureProvider is reversible test framing, not encryption.
type maskingFixtureProvider struct{ calls atomic.Int32 }

func (p *maskingFixtureProvider) check(ctx context.Context, aad map[string]string) error {
	p.calls.Add(1)
	lc, ok := lambdacontext.FromContext(ctx)
	if !ok || lc.AwsRequestID == "" || aad["tenant"] != "fixture" {
		return errors.New("masking provider lost invocation or authenticated context")
	}
	return ctx.Err()
}

func (p *maskingFixtureProvider) Encrypt(ctx context.Context, data string, aad map[string]string) (string, error) {
	return "fixture:" + data, p.check(ctx, aad)
}
func (p *maskingFixtureProvider) Decrypt(ctx context.Context, data string, aad map[string]string) (string, error) {
	if !strings.HasPrefix(data, "fixture:") {
		return "", errors.New("invalid masking fixture")
	}
	return strings.TrimPrefix(data, "fixture:"), p.check(ctx, aad)
}

func dataMaskingProbe(ctx context.Context) (map[string]any, error) {
	provider := &maskingFixtureProvider{}
	warnings := []string{}
	masker := datamasking.New(datamasking.Config{Provider: provider, IgnoreMissing: true, Warn: func(_ context.Context, message string) { warnings = append(warnings, message) }})
	input := map[string]any{"users": []any{map[string]any{"secret": "one"}, map[string]any{"secret": nil}}, "public": "visible"}
	dynamic, custom := true, "[redacted]"
	masked, err := masker.Erase(ctx, input, datamasking.EraseOptions{
		Fields: []string{"users[*].secret", "missing"},
		Rule:   datamasking.Rule{DynamicMask: &dynamic},
		Rules:  []datamasking.FieldRule{{Field: "users.0.secret", Rule: datamasking.Rule{CustomMask: &custom}}},
	})
	if err != nil {
		return nil, err
	}
	options := datamasking.TransformOptions{Fields: []string{"users[*].secret"}, Context: map[string]string{"tenant": "fixture"}}
	encrypted, err := masker.Encrypt(ctx, input, options)
	if err != nil {
		return nil, err
	}
	restored, err := masker.Decrypt(ctx, encrypted, options)
	if err != nil {
		return nil, err
	}
	_, err = datamasking.New(datamasking.Config{}).Encrypt(ctx, input, datamasking.TransformOptions{})
	var failure *datamasking.Error
	missing := errors.As(err, &failure) && failure.ErrorName() == "DataMaskingEncryptionError"
	expression, err := regex.Compile(`(?<name>\p{Letter}+)`, "gu", regex.Options{})
	if err != nil {
		return nil, err
	}
	regexMasked, err := masker.Erase(ctx, map[string]any{"secret": "αβ one"}, datamasking.EraseOptions{Fields: []string{"secret"}, Rule: datamasking.Rule{Replace: expression.Replacer("[$<name>]")}})
	if err != nil {
		return nil, err
	}
	sticky, err := regex.Compile(`.`, "uy", regex.Options{})
	if err != nil {
		return nil, err
	}
	sticky.SetLastIndex(1)
	stickyValue, err := sticky.Replace("😀ab", "#")
	if err != nil {
		return nil, err
	}
	kmsResult, err := kmsMaskingProbe(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"kms": kmsResult, "masked": masked, "restored": restored, "original": input, "warnings": warnings, "provider_calls": provider.calls.Load(), "missing_provider": missing, "regex": regexMasked, "regex_index": expression.LastIndex(), "sticky": stickyValue, "sticky_index": sticky.LastIndex()}, nil
}
