// Package kmsfixture supplies a local, non-cryptographic KMS service fixture.
// Only the real Encryption SDK encrypts message content. Fixture wrapped keys
// contain plaintext test material and must never be used outside local tests.
package kmsfixture

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
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
	if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("X-Amz-Target"), "TrentService.") {
		serviceError(w, "UnknownOperationException", "unsupported local fixture protocol")
		return
	}
	operation := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "TrentService.")
	f.mu.Lock()
	if f.Calls == nil {
		f.Calls = map[string]int{}
	}
	f.Calls[operation]++
	f.UserAgents = append(f.UserAgents, r.Header.Get("User-Agent"))
	f.mu.Unlock()
	var input struct {
		KeyId                        string
		KeySpec, EncryptionAlgorithm string
		NumberOfBytes                *int
		Plaintext, CiphertextBlob    []byte
		EncryptionContext            map[string]string
		GrantTokens                  []string
		DryRun                       bool
		Recipient                    json.RawMessage
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var trailing any
	if err := decoder.Decode(&input); err != nil {
		serviceError(w, "ValidationException", "invalid synthetic fixture JSON request")
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		serviceError(w, "ValidationException", "expected one JSON request")
		return
	}
	if len(input.GrantTokens) != 0 || input.DryRun || len(input.Recipient) != 0 {
		serviceError(w, "ValidationException", "unsupported local fixture options")
		return
	}
	if input.EncryptionAlgorithm != "" && input.EncryptionAlgorithm != "SYMMETRIC_DEFAULT" {
		serviceError(w, "InvalidKeyUsageException", "fixture supports symmetric keys only")
		return
	}
	var result any
	switch operation {
	case "GenerateDataKey", "Encrypt":
		if input.KeyId == "" {
			serviceError(w, "ValidationException", "KeyId is required")
			return
		}
		key := input.Plaintext
		if operation == "GenerateDataKey" {
			length := 0
			if (input.NumberOfBytes != nil) == (input.KeySpec != "") {
				serviceError(w, "ValidationException", "specify exactly one of KeySpec and NumberOfBytes")
				return
			}
			if input.NumberOfBytes != nil {
				length = *input.NumberOfBytes
			} else {
				switch input.KeySpec {
				case "AES_128":
					length = 16
				case "AES_256":
					length = 32
				}
			}
			if length < 1 || length > 1024 {
				serviceError(w, "ValidationException", "invalid data key length or KeySpec")
				return
			}
			key = make([]byte, length)
			if _, err := rand.Read(key); err != nil {
				panic(err)
			}
		} else if len(key) < 1 || len(key) > 4096 {
			serviceError(w, "ValidationException", "invalid plaintext length")
			return
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
		if err := json.Unmarshal(input.CiphertextBlob, &wrapped); err != nil || wrapped.Key == "" || len(wrapped.Plaintext) == 0 || !maps.Equal(wrapped.Context, input.EncryptionContext) {
			serviceError(w, "InvalidCiphertextException", "invalid fixture wrapped key or context")
			return
		}
		if input.KeyId != "" && wrapped.Key != input.KeyId {
			serviceError(w, "IncorrectKeyException", "fixture wrapped key belongs to another key")
			return
		}
		result = map[string]any{"KeyId": wrapped.Key, "Plaintext": wrapped.Plaintext, "EncryptionAlgorithm": "SYMMETRIC_DEFAULT"}
	default:
		serviceError(w, "UnknownOperationException", fmt.Sprintf("unexpected fixture operation %q", operation))
		return
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	json.NewEncoder(w).Encode(result)
}

func serviceError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": message})
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
func Clients(endpoint string, options ...func(*awskms.Options)) func(context.Context, string) (*awskms.Client, error) {
	return func(_ context.Context, region string) (*awskms.Client, error) {
		return awskms.New(awskms.Options{Region: region, BaseEndpoint: aws.String(endpoint), Credentials: credentials.NewStaticCredentialsProvider("fixture", "fixture", ""), RetryMaxAttempts: 1}, options...), nil
	}
}
