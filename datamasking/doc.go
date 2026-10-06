// Package datamasking erases selected data and composes encryption providers.
//
// [New] creates a [Masker]. [Masker.Erase] replaces fields selected by
// [EraseOptions] using the default mask, a custom mask, a dynamic mask or a
// replacement callback. Ordered [FieldRule] entries control overlapping selectors.
// Operations work on private copies rather than mutating caller-owned payloads.
//
// # Providers and missing fields
//
// Encryption and decryption use an application-supplied [Provider], which owns
// ciphertext format and authenticated context. The optional datamasking/kms module
// implements that interface with the AWS Encryption SDK; erasure needs no provider.
//
// [Config.IgnoreMissing] selects warning behavior for absent selectors. Strict
// mode reports missing fields. Null and [Undefined] retain their documented
// semantics. Review the guide for supported input types and selector syntax.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/DATAMASKING.md
package datamasking
