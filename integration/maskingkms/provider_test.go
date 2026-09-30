package maskingkms_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
	maskkms "github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
	"github.com/rambow-cloud/powertools-lambda-go/integration/internal/kmsfixture"
)

func TestTypeScriptCiphertexts(t *testing.T) {
	var corpus struct {
		Cases []struct {
			Name, Ciphertext, Expected string
			Keys                       []string
			Context                    map[string]string
			Checks                     []struct {
				Context map[string]string
				Value   string
				Error   *struct{ Name, Message string }
			}
		}
		Failures []struct {
			Ciphertext string
			Error      any
		}
	}
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	fixture := &kmsfixture.Fixture{}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			provider, err := maskkms.New(context.Background(), maskkms.Config{Keys: item.Keys, ClientProvider: kmsfixture.Clients(server.URL)})
			if err != nil {
				t.Fatal(err)
			}
			for _, ciphertext := range []string{item.Ciphertext, strings.TrimRight(item.Ciphertext, "="), strings.NewReplacer("+", "-", "/", "_").Replace(item.Ciphertext), "! \n" + item.Ciphertext + "ignored-after-padding"} {
				// Trailing data is ignored only after an actual padding delimiter.
				if strings.HasSuffix(ciphertext, "ignored-after-padding") && !strings.Contains(item.Ciphertext, "=") {
					ciphertext = "! \n" + item.Ciphertext
				}
				value, err := provider.Decrypt(context.Background(), ciphertext, item.Context)
				if err != nil || value != item.Expected {
					t.Fatalf("decryption: %q, %v; expected %q", value, err, item.Expected)
				}
			}
			for _, check := range item.Checks {
				value, err := provider.Decrypt(context.Background(), item.Ciphertext, check.Context)
				if check.Error == nil {
					if err != nil || value != check.Value {
						t.Errorf("context %#v: %q, %v", check.Context, value, err)
					}
				} else {
					var failure *datamasking.Error
					if !errors.As(err, &failure) || failure.ErrorName() != check.Error.Name || failure.Error() != check.Error.Message || value != "" {
						t.Errorf("context error: got %v, want %+v", err, check.Error)
					}
				}
			}
			if len(item.Keys) > 1 {
				secondary, err := maskkms.New(context.Background(), maskkms.Config{Keys: item.Keys[1:], ClientProvider: kmsfixture.Clients(server.URL)})
				if err != nil {
					t.Fatal(err)
				}
				value, err := secondary.Decrypt(context.Background(), item.Ciphertext, item.Context)
				if err != nil || value != item.Expected {
					t.Errorf("secondary key: %q, %v", value, err)
				}
			}
		})
	}
	provider, err := maskkms.New(context.Background(), maskkms.Config{Keys: []string{kmsfixture.Key}, ClientProvider: kmsfixture.Clients(server.URL)})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range corpus.Failures {
		if item.Error == nil {
			t.Fatal("reference failure fixture unexpectedly succeeded")
		}
		if value, err := provider.Decrypt(context.Background(), item.Ciphertext, nil); err == nil || value != "" {
			t.Errorf("accepted malformed ciphertext: %q, %v", value, err)
		}
	}
}

type invocationKey struct{}
type checkContextClient struct {
	t        *testing.T
	expected any
}

func (c checkContextClient) Do(request *http.Request) (*http.Response, error) {
	if request.Context().Value(invocationKey{}) != c.expected {
		c.t.Error("KMS request lost invocation context")
	}
	if _, ok := request.Context().Deadline(); !ok {
		c.t.Error("KMS request lost deadline")
	}
	return http.DefaultClient.Do(request)
}

func TestConcurrentEncryptionOwnershipAndContext(t *testing.T) {
	fixture := &kmsfixture.Fixture{}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	keys := []string{kmsfixture.Key, kmsfixture.OtherKey}
	provider, err := maskkms.New(context.Background(), maskkms.Config{Keys: keys, ClientProvider: func(ctx context.Context, region string) (*awskms.Client, error) {
		return kmsfixture.Clients(server.URL, func(options *awskms.Options) {
			options.HTTPClient = checkContextClient{t: t, expected: ctx.Value(invocationKey{})}
		})(ctx, region)
	}})
	if err != nil {
		t.Fatal(err)
	}
	keys[0] = "changed-after-construction"
	var workers sync.WaitGroup
	for i := range 32 {
		workers.Go(func() {
			ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), invocationKey{}, i), 30*time.Second)
			defer cancel()
			aad := map[string]string{"tenant": "example"}
			ciphertext, err := provider.Encrypt(ctx, "αβ 😀", aad)
			if err != nil {
				t.Error(err)
				return
			}
			plain, err := provider.Decrypt(ctx, ciphertext, aad)
			if err != nil || plain != "αβ 😀" {
				t.Errorf("round trip %q: %v", plain, err)
			}
			if !reflect.DeepEqual(aad, map[string]string{"tenant": "example"}) {
				t.Error("encryption mutated caller context")
			}
		})
	}
	workers.Wait()
	calls, agents := fixture.Snapshot()
	if calls["GenerateDataKey"] != 32 || calls["Encrypt"] != 32 || calls["Decrypt"] != 32 {
		t.Errorf("unexpected uncached KMS calls: %v", calls)
	}
	for _, agent := range agents {
		if !strings.Contains(agent, "PT/data-masking/") {
			t.Errorf("SDK identity absent: %q", agent)
		}
	}
}

func TestCancellationDuringKMS(t *testing.T) {
	entered := make(chan struct{}, 1)
	observed := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		entered <- struct{}{}
		select {
		case <-r.Context().Done():
			close(observed)
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })
	provider, err := maskkms.New(context.Background(), maskkms.Config{Keys: []string{kmsfixture.Key}, ClientProvider: kmsfixture.Clients(server.URL)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	finished := make(chan error, 1)
	go func() { _, err := provider.Encrypt(ctx, "secret", nil); finished <- err }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("KMS request did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("cancellation identity lost: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("KMS operation ignored cancellation")
	}
	select {
	case <-observed:
	case <-time.After(5 * time.Second):
		t.Error("server did not observe cancellation")
	}
	if _, err := provider.Decrypt(ctx, "", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("pre-canceled decrypt: %v", err)
	}
}
