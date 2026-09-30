// Package kmsfixture supplies a local, non-cryptographic KMS service fixture.
// Only the real Encryption SDK encrypts message content. Fixture wrapped keys
// contain plaintext test material and must never be used outside local tests.
package kmsfixture

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	maskkms "github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
)

const Key = "arn:aws:kms:ap-east-1:111122223333:key/11111111-1111-4111-8111-111111111111"
const OtherKey = "arn:aws:kms:us-east-1:111122223333:key/22222222-2222-4222-8222-222222222222"

type Fixture struct {
	mu         sync.Mutex
	Calls      map[string]int
	UserAgents []string
}

type wrapping struct {
	Key       string            `json:"key"`
	Plaintext []byte            `json:"plaintext"`
	Context   map[string]string `json:"context"`
}

func (f *Fixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	operation := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "TrentService.")
	f.mu.Lock()
	if f.Calls == nil {
		f.Calls = map[string]int{}
	}
	f.Calls[operation]++
	f.UserAgents = append(f.UserAgents, r.Header.Get("User-Agent"))
	f.mu.Unlock()
	var input struct {
		KeyId                     string
		NumberOfBytes             int
		Plaintext, CiphertextBlob []byte
		EncryptionContext         map[string]string
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var result any
	switch operation {
	case "GenerateDataKey", "Encrypt":
		key := input.Plaintext
		if operation == "GenerateDataKey" {
			key = make([]byte, input.NumberOfBytes)
			if _, err := rand.Read(key); err != nil {
				panic(err)
			}
		}
		blob, err := json.Marshal(wrapping{Key: input.KeyId, Plaintext: key, Context: input.EncryptionContext})
		if err != nil {
			panic(err)
		}
		result = map[string]any{"KeyId": input.KeyId, "CiphertextBlob": blob}
		if operation == "GenerateDataKey" {
			result.(map[string]any)["Plaintext"] = key
		}
	case "Decrypt":
		var wrapped wrapping
		if err := json.Unmarshal(input.CiphertextBlob, &wrapped); err != nil || wrapped.Key != input.KeyId || !reflect.DeepEqual(wrapped.Context, input.EncryptionContext) {
			w.Header().Set("Content-Type", "application/x-amz-json-1.1")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"__type": "InvalidCiphertextException", "message": "invalid fixture wrapped key or context"})
			return
		}
		result = map[string]any{"KeyId": wrapped.Key, "Plaintext": wrapped.Plaintext, "EncryptionAlgorithm": "SYMMETRIC_DEFAULT"}
	default:
		http.Error(w, fmt.Sprintf("unexpected fixture operation %q", operation), 400)
		return
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	json.NewEncoder(w).Encode(result)
}

func (f *Fixture) Snapshot() (map[string]int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := map[string]int{}
	for key, value := range f.Calls {
		result[key] = value
	}
	return result, append([]string(nil), f.UserAgents...)
}

// Clients routes regional SDK calls only to the supplied loopback endpoint.
func Clients(endpoint string, options ...func(*awskms.Options)) maskkms.ClientProvider {
	return func(_ context.Context, region string) (*awskms.Client, error) {
		return awskms.New(awskms.Options{Region: region, BaseEndpoint: aws.String(endpoint), Credentials: credentials.NewStaticCredentialsProvider("fixture", "fixture", ""), RetryMaxAttempts: 1}, options...), nil
	}
}
