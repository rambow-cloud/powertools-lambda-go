package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// Verify only synthetic fixture requests using the underlying AWS implementation.
// This does not simulate IAM authorization or real S3 service behavior.
func verifyFixtureSignature(request *http.Request) bool {
	want := request.Header.Get("Authorization")
	_, fields, found := strings.Cut(want, "SignedHeaders=")
	if !found {
		return false
	}
	signedHeaders, _, found := strings.Cut(fields, ",")
	if !found {
		return false
	}
	copy := request.Clone(request.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = request.Host
	copy.Header = make(http.Header)
	for _, name := range strings.Split(signedHeaders, ";") {
		if name != "host" {
			copy.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), request.Header.Values(name)...)
		}
	}
	date, err := time.Parse("20060102T150405Z", request.Header.Get("X-Amz-Date"))
	if err != nil {
		return false
	}
	sum := sha256.Sum256(nil)
	hash := hex.EncodeToString(sum[:])
	if request.Header.Get("X-Amz-Content-Sha256") != hash {
		return false
	}
	credentials := aws.Credentials{AccessKeyID: "LOCALTESTONLY", SecretAccessKey: "local-test-only"}
	if err = v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true }).SignHTTP(request.Context(), credentials, copy, hash, "s3", "ap-east-1", date); err != nil {
		return false
	}
	return copy.Header.Get("Authorization") == want
}
