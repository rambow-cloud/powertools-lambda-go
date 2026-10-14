package maskingkms_test

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	maskkms "github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
	"github.com/rambow-cloud/powertools-lambda-go/integration/internal/kmsfixture"
)

func TestClientSupplierFailuresReturnNoCiphertext(t *testing.T) {
	cause := errors.New("local client factory failed")
	for _, nilClient := range []bool{false, true} {
		calls := 0
		ctx := context.WithValue(context.Background(), invocationKey{}, "owned invocation")
		provider, err := maskkms.New(ctx, maskkms.Config{Keys: []string{kmsfixture.Key}, ClientProvider: func(got context.Context, _ string) (*awskms.Client, error) {
			calls++
			if got != ctx {
				t.Error("factory lost invocation context")
			}
			if nilClient {
				return nil, nil
			}
			return nil, cause
		}})
		if err == nil {
			var ciphertext string
			ciphertext, err = provider.Encrypt(ctx, "secret", nil)
			if ciphertext != "" {
				t.Fatal("factory failure returned ciphertext")
			}
		}
		if err == nil || calls == 0 {
			t.Fatalf("factory failure or nil client ignored: %v, calls=%d", err, calls)
		}
		if !nilClient && !errors.Is(err, cause) {
			t.Fatalf("client factory cause lost across MPL bridge: %v", err)
		}
	}
}

func TestInvalidMessageHeadersFailBeforeKMSRequests(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Ciphertext string `json:"ciphertext"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) == 0 {
		t.Fatalf("invalid pinned ciphertext corpus: %v", err)
	}
	wire, err := base64.StdEncoding.DecodeString(fixture.Cases[0].Ciphertext)
	if err != nil || len(wire) < 3 || wire[0] != 2 {
		t.Fatal("expected a version-2 committed message")
	}
	unknownSuite := append([]byte(nil), wire...)
	unknownSuite[1], unknownSuite[2] = 0xff, 0xff
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)
	provider, err := maskkms.New(context.Background(), maskkms.Config{Keys: []string{kmsfixture.Key}, ClientProvider: kmsfixture.Clients(server.URL)})
	if err != nil {
		t.Fatal(err)
	}
	for _, ciphertext := range []string{"", "AA==", "Ag==", base64.StdEncoding.EncodeToString(unknownSuite)} {
		value, err := provider.Decrypt(context.Background(), ciphertext, nil)
		if err == nil || value != "" {
			t.Errorf("invalid header returned plaintext: %q, %v", value, err)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid message headers made %d KMS requests", requests.Load())
	}
}
