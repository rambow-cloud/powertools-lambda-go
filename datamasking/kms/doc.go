// Package kms supplies an AWS Encryption SDK provider for Data Masking.
//
// [New] constructs a [Provider] from key identifiers in [Config]. The first key
// is the generator; additional keys wrap the same data key. [Provider.Encrypt]
// and [Provider.Decrypt] exchange Base64-encoded authenticated Encryption SDK
// messages, rather than raw KMS Encrypt ciphertext.
//
// # Key and context policy
//
// The provider requires key commitment. Encryption context is authenticated;
// decryption checks the caller's expected context. [ClientProvider] lets applications
// supply regional AWS SDK clients while retaining invocation cancellation.
// Actual cryptographic operations require KMS access and permissions.
//
// Each encryption obtains fresh materials. TypeScript data-key caching and its
// age, capacity, message and byte limits are not implemented. This optional module
// keeps Encryption SDK dependencies out of the core erasure package.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/DATAMASKING_KMS.md
package kms
