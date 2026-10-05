# Local KMS HTTP fixture

This synthetic fixture supports the symmetric `GenerateDataKey`, `Encrypt` and
`Decrypt` calls used by the integration tests. It runs real AWS SDK requests and
JSON/base64 responses. Request constraints come from the official
[GenerateDataKey](https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html),
[Encrypt](https://docs.aws.amazon.com/kms/latest/APIReference/API_Encrypt.html) and
[Decrypt](https://docs.aws.amazon.com/kms/latest/APIReference/API_Decrypt.html)
contracts, reviewed on 2026-10-06. The SDK is pinned by `integration/go.mod`.

GenerateDataKey accepts exactly one length selector: `NumberOfBytes` (1–1024) or
`KeySpec` (`AES_128`/`AES_256`). Encrypt accepts 1–4096 bytes. Decrypt checks the
synthetic key identity and exact context; a symmetric KeyId may be omitted.
Absent and empty contexts both represent no context entries.
Unsupported protocol/operation/input cases return explicit JSON error envelopes.
These are deterministic fixture policies, not recordings of AWS error responses.
Unknown fields and active grant-token/recipient/dry-run options fail explicitly;
empty grant-token lists retain ordinary symmetric request compatibility.

The wrapped key contains plaintext test material in JSON. It is not a KMS
ciphertext. The real Encryption SDK encrypts message content; this fixture proves
local SDK encoding, composition and context binding. IAM, genuine KMS encryption,
grant tokens, recipients, dry runs, key lifecycle and asymmetric keys require a
separately authorized cloud suite.

With `CGO_ENABLED=0`, run `go test ./integration/internal/kmsfixture` from the
repository root. The fixture has no Encryption SDK import, so these HTTP contract
tests also run natively on Windows. Full Encryption SDK interoperability remains
part of Linux integration acceptance.
