---
description: "Encrypt and decrypt masked Go payload fields with the optional Powertools KMS provider and documented AWS Encryption SDK interoperability."
---

# Encrypt selected fields with KMS

Use the optional KMS provider to encrypt and decrypt fields with the AWS Encryption SDK. Masking without encryption does not require this module.

## Encrypt and restore a customer secret

Install `datamasking/kms@v1.3.0`. The fragment assumes `ctx`, a payload, and imports for `datamasking`, `os` and `maskkms` (`datamasking/kms`). It calls KMS: set `KMS_KEY_ARN` to a full key ARN, configure the AWS SDK, and grant the required data-key/decrypt permissions.

**Input**

```json
{
  "customer": {
    "secret": "Ada"
  }
}
```

**Code**

```go
provider, err := maskkms.New(ctx, maskkms.Config{Keys: []string{os.Getenv("KMS_KEY_ARN")}})
if err != nil { return err }
masker := datamasking.New(datamasking.Config{Provider: provider})
options := datamasking.TransformOptions{
    Fields:  []string{"customer.secret"},
    Context: map[string]string{"tenant": "example"},
}
encrypted, err := masker.Encrypt(ctx, payload, options)
if err != nil { return err }
restored, err := masker.Decrypt(ctx, encrypted, options)
if err != nil { return err }
```

**Output**

```json
{
  "customer": {
    "secret": "Ada"
  }
}
```

This is `restored`. In `encrypted`, `customer.secret` is a Base64 AWS Encryption SDK message. Ciphertext is different on each encryption; it is not a mask string. Decryption checks the authenticated context before returning the plaintext.

## Reuse the provider

Create one provider outside the handler. Supply `ClientProvider` when you need application-owned regional KMS clients, endpoints or tracing configuration. With multiple key ARNs, the first generates a data key and additional keys protect it for other recipients.

## Things to know

- The provider currently has no data-key caching. Encryption makes fresh data-key requests.
- Encryption context is authenticated data, not a secret: do not put sensitive values in it.
- Use Linux for this module. The pinned upstream SDK cannot build as a native Windows binary; Lambda amd64 and arm64 are supported build targets.
- Keep the invocation context for cancellation and deadlines. Cryptographic computation inside the SDK cannot be forcibly interrupted.

## Next steps

[API and behavior notes](reference/DATAMASKING_KMS.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
