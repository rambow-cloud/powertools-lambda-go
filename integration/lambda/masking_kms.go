package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
	maskkms "github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
	"github.com/rambow-cloud/powertools-lambda-go/integration/internal/kmsfixture"
)

func kmsMaskingProbe(ctx context.Context) (map[string]any, error) {
	fixture := &kmsfixture.Fixture{}
	server := httptest.NewServer(fixture)
	defer server.Close()
	provider, err := maskkms.New(ctx, maskkms.Config{Keys: []string{kmsfixture.Key}, ClientProvider: kmsfixture.Clients(server.URL)})
	if err != nil {
		return nil, err
	}
	masker := datamasking.New(datamasking.Config{Provider: provider})
	original := map[string]any{"secret": "αβ 😀", "public": "visible"}
	options := datamasking.TransformOptions{Fields: []string{"secret"}, Context: map[string]string{"tenant": "fixture"}}
	encrypted, err := masker.Encrypt(ctx, original, options)
	if err != nil {
		return nil, err
	}
	second, err := masker.Encrypt(ctx, original, options)
	if err != nil {
		return nil, err
	}
	restored, err := masker.Decrypt(ctx, encrypted, options)
	if err != nil {
		return nil, err
	}
	_, err = masker.Decrypt(ctx, encrypted, datamasking.TransformOptions{Fields: []string{"secret"}, Context: map[string]string{"tenant": "wrong"}})
	var mismatch *datamasking.Error
	contextMismatch := errors.As(err, &mismatch) && mismatch.ErrorName() == "DataMaskingEncryptionError" && mismatch.Error() == "Encryption context mismatch for key 'tenant'"
	calls, agents := fixture.Snapshot()
	identity := len(agents) == 4
	for _, agent := range agents {
		identity = identity && strings.Contains(agent, "PT/data-masking/")
	}
	return map[string]any{"restored": restored, "original": original, "fresh": !reflect.DeepEqual(encrypted, second), "calls": calls, "context_mismatch": contextMismatch, "identity": identity}, nil
}
