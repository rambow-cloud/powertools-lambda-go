package kmsfixture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

func sdkClient(t *testing.T) *awskms.Client {
	t.Helper()
	server := httptest.NewServer(&Fixture{})
	t.Cleanup(server.Close)
	client, err := Clients(server.URL)(context.Background(), "ap-east-1")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestSDKDataKeyContracts(t *testing.T) {
	client := sdkClient(t)
	ctx := context.Background()
	for _, item := range []struct {
		name   string
		bytes  *int32
		spec   types.DataKeySpec
		length int
	}{
		{"bytes", aws.Int32(32), "", 32}, {"minimum", aws.Int32(1), "", 1}, {"maximum", aws.Int32(1024), "", 1024},
		{"aes128", nil, types.DataKeySpecAes128, 16}, {"aes256", nil, types.DataKeySpecAes256, 32},
	} {
		t.Run(item.name, func(t *testing.T) {
			contextMap := map[string]string{"service": "synthetic"}
			out, err := client.GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{KeyId: aws.String(Key), NumberOfBytes: item.bytes, KeySpec: item.spec, EncryptionContext: contextMap})
			if err != nil || len(out.Plaintext) != item.length || len(out.CiphertextBlob) == 0 || aws.ToString(out.KeyId) != Key {
				t.Fatalf("data key: %+v/%v", out, err)
			}
			plain, err := client.Decrypt(ctx, &awskms.DecryptInput{CiphertextBlob: out.CiphertextBlob, EncryptionContext: contextMap})
			if err != nil || !bytes.Equal(plain.Plaintext, out.Plaintext) || plain.EncryptionAlgorithm != types.EncryptionAlgorithmSpecSymmetricDefault {
				t.Fatalf("binary roundtrip: %+v/%v", plain, err)
			}
			_, err = client.Decrypt(ctx, &awskms.DecryptInput{KeyId: aws.String(Key), CiphertextBlob: out.CiphertextBlob, EncryptionContext: map[string]string{"service": "wrong"}})
			var invalid *types.InvalidCiphertextException
			if !errors.As(err, &invalid) {
				t.Fatalf("wrong context not typed: %v", err)
			}
		})
	}
}

func TestSDKInvalidDataKeyInputs(t *testing.T) {
	client := sdkClient(t)
	for _, item := range []struct {
		name  string
		bytes *int32
		spec  types.DataKeySpec
	}{
		{"missing", nil, ""}, {"both", aws.Int32(32), types.DataKeySpecAes256}, {"zero", aws.Int32(0), ""},
		{"negative", aws.Int32(-1), ""}, {"too-large", aws.Int32(1025), ""}, {"bad-spec", nil, "unsupported"},
	} {
		t.Run(item.name, func(t *testing.T) {
			_, err := client.GenerateDataKey(context.Background(), &awskms.GenerateDataKeyInput{KeyId: aws.String(Key), NumberOfBytes: item.bytes, KeySpec: item.spec})
			var api interface {
				error
				ErrorCode() string
			}
			if !errors.As(err, &api) || api.ErrorCode() != "ValidationException" {
				t.Fatalf("invalid request accepted or untyped: %v", err)
			}
		})
	}
}

func TestSDKEncryptAndWrongKey(t *testing.T) {
	client := sdkClient(t)
	ctx := context.Background()
	want := []byte{0, 1, 255, 0, 128}
	out, err := client.Encrypt(ctx, &awskms.EncryptInput{KeyId: aws.String(Key), Plaintext: want})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := client.Decrypt(ctx, &awskms.DecryptInput{KeyId: aws.String(Key), CiphertextBlob: out.CiphertextBlob, EncryptionContext: map[string]string{}})
	if err != nil || !bytes.Equal(plain.Plaintext, want) {
		t.Fatalf("encrypted bytes: %+v/%v", plain, err)
	}
	_, err = client.Decrypt(ctx, &awskms.DecryptInput{KeyId: aws.String(OtherKey), CiphertextBlob: out.CiphertextBlob})
	var incorrect *types.IncorrectKeyException
	if !errors.As(err, &incorrect) {
		t.Fatalf("wrong key: %v", err)
	}
	_, err = client.Encrypt(ctx, &awskms.EncryptInput{KeyId: aws.String(Key), Plaintext: want, EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256})
	var usage *types.InvalidKeyUsageException
	if !errors.As(err, &usage) {
		t.Fatalf("unsupported key algorithm: %v", err)
	}
}

func TestFixtureRejectsUnsupportedAndMalformedHTTP(t *testing.T) {
	for _, item := range []struct{ name, method, target, body string }{
		{"method", "GET", "TrentService.Encrypt", "{}"}, {"target", "POST", "other.Encrypt", "{}"},
		{"operation", "POST", "TrentService.Unknown", "{}"}, {"malformed", "POST", "TrentService.Encrypt", "{"},
		{"trailing", "POST", "TrentService.Encrypt", "{} {}"}, {"empty-key", "POST", "TrentService.Encrypt", `{"Plaintext":"AA=="}`},
		{"empty-plaintext", "POST", "TrentService.Encrypt", `{"KeyId":"fixture"}`},
		{"unknown-field", "POST", "TrentService.Encrypt", `{"KeyId":"fixture","Plaintext":"AA==","extra":true}`},
		{"dry-run", "POST", "TrentService.Encrypt", `{"KeyId":"fixture","Plaintext":"AA==","DryRun":true}`},
		{"grant", "POST", "TrentService.Encrypt", `{"KeyId":"fixture","Plaintext":"AA==","GrantTokens":["synthetic"]}`},
		{"recipient", "POST", "TrentService.GenerateDataKey", `{"KeyId":"fixture","NumberOfBytes":32,"Recipient":{}}`},
	} {
		t.Run(item.name, func(t *testing.T) {
			r := httptest.NewRequest(item.method, "/", strings.NewReader(item.body))
			r.Header.Set("X-Amz-Target", item.target)
			w := httptest.NewRecorder()
			(&Fixture{}).ServeHTTP(w, r)
			response := w.Result()
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			if response.StatusCode != http.StatusBadRequest || response.Header.Get("Content-Type") != "application/x-amz-json-1.1" || !bytes.Contains(body, []byte(`"__type"`)) {
				t.Fatalf("unstructured fixture failure: %d %s", response.StatusCode, body)
			}
		})
	}
}
