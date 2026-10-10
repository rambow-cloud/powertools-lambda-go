---
description: "Encrypt and decrypt masked Go payload fields with the optional Powertools KMS provider and documented AWS Encryption SDK interoperability."
---

# AWS Encryption SDK masking provider

The optional datamasking/kms module implements datamasking.Provider with the official AWS Encryption SDK for Go v0.4.0 and Materials Providers Library v0.4.0. It emits Base64 AWS Encryption SDK messages with the same committed, signed default suite (0x0578) as the pinned TypeScript provider. Root Commons and ordinary Data Masking do not acquire these dependencies.

This initial provider is **uncached**. TypeScript enables a data-key caching materials manager by default. Cache capacity, age, message and byte limits remain an explicit compatibility gap; no ignored cache options are exposed.

## Usage

```go
import (
    "github.com/rambow-cloud/powertools-lambda-go/datamasking"
    maskkms "github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
)

provider, err := maskkms.New(ctx, maskkms.Config{
    Keys: []string{
        "arn:aws:kms:ap-east-1:111122223333:key/11111111-1111-4111-8111-111111111111",
    },
})
if err != nil {
    return err
}
masker := datamasking.New(datamasking.Config{Provider: provider})
encrypted, err := masker.Encrypt(ctx, payload, datamasking.TransformOptions{
    Fields: []string{"customer.secret"},
    Context: map[string]string{"tenant": "example"},
})
```

The first key is the generator; subsequent keys protect the same data key as additional recipients. Use complete KMS key ARNs for encrypt/decrypt configurations. The constructor copies the key list and validates keyring construction. Default clients use the standard AWS configuration; ClientProvider can supply application-owned regional SDK client templates, including credentials, transport and tracing middleware. Templates are not mutated. Construct one provider and reuse it across invocations.

Both encryption and decryption require key commitment. Decryption accepts the SDK's supported committed message suites and verifies each requested encryption-context entry against authenticated output before returning plaintext. Missing expected entries fail even when the expected value is empty. Context mismatches return DataMaskingEncryptionError with the upstream message. SDK failures retain native SDK error types and wording.

The provider reuses Commons permissive Buffer Base64 decoding, UTF-8 decoding, object-key ordering and AWS SDK request identity. Decryption strips one leading UTF-8 BOM, matching TextDecoder. Plaintext strings are normalized as UTF-8; arbitrary invalid Go UTF-8 and WTF-8 lone surrogates are not fully equivalent to JavaScript TextEncoder. Multiple mismatching context entries follow deterministic native map ordering, not JavaScript insertion order.

## Context and platform behavior

The v0.4.0 generated ESDK/MPL bridge discards context.Context before calling KMS. The adapter constructs an operation-scoped client supplier and restores invocation cancellation, deadlines and values at the AWS SDK initialization boundary while retaining SDK stack metadata. This covers signing, retries and network I/O without global mutable invocation state. Calls check cancellation before and after SDK execution; pure cryptographic computation inside the upstream SDK is not interruptible.

The upstream smithy-dafny-standard-library v0.4.0 calls syscall.Getrusage without Windows build constraints. Consequently, this provider cannot compile as a native Windows binary. Lambda's Linux amd64/arm64 target remains supported. Do not patch the module cache or substitute cryptography to hide this dependency limit.

tools/modules.json marks this module for Linux verification. tools/modules.py propagates that target to dependent modules, uses local Go cross-compilation and runs test binaries in the pinned provided.al2023 Docker image on Windows. On Linux it runs them natively. tools/run-linux-test.py maps the read-only workspace and writable Go test-artifact directory; test networking is limited to the container's loopback interface. Standalone consumers are built for Linux and the report records their platform.

## Local cryptographic acceptance

The fixture corpus contains 39 real TypeScript SDK encrypted-message cases, including empty, Unicode, BOM, multi-frame and malformed UTF-8 plaintext, one/two wrapping keys and authenticated-context checks. Five rejection cases cover empty, truncated, malformed and signature-damaged messages. Go tests also cover Base64 variants, secondary-key decryption, 32 concurrent invocations, input ownership, uncached request counts and SDK identity.

The complementary bridge creates Go messages with real encryption and verifies 108 assertions using the actual TypeScript v2.35.0 provider with @aws-crypto/client-node v5.0.2: plaintext, committed signed suite and context mismatch. DATAMASKING_KMS_INTEROP.json records that result. Native tests compile on the host and execute in Linux Docker.

The local KMS fixtures intentionally serialize plaintext test data keys into fixture wrapping blobs. They prove Encryption SDK message interoperability and authenticated content handling, **not** real KMS wrapping, IAM, key policy or service behavior. They use dummy credentials and never contact AWS. All fixtures live in the development integration module or reference generator.

The first native Windows compile exposed the upstream syscall limitation. The first Linux suite reached a test-server cleanup timeout after client cancellation succeeded; the fixture now consumes the request body and explicitly verifies server-observed cancellation. The scoped package check in checkpoint-06 passes the KMS, integration and tools modules, including GOWORK=off tests/vet/tidy and the independent Linux consumer. Combined evidence covers 31 modules/28 consumers in MODULE_ACCEPTANCE_KMS.json. Both CGO-disabled Linux builds pass; Lambda RIE passes 868/868 assertions, streaming passes 95/95 and saved Batch checks pass 14/14, with temporary resources cleaned. Twelve new RIE assertions exercise actual SDK message round trips, caller ownership, fresh data keys, context rejection and SDK identity. Docker executes amd64; arm64 is cross-compiled. Full cache, identifier/algorithm/error/native parity, service and performance gates remain open.
