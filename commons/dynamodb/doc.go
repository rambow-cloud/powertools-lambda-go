// Package dynamodb converts AWS SDK v2 DynamoDB attributes to Go values.
//
// [UnmarshalAttribute] decodes one attribute and [UnmarshalItem] decodes an item.
// [Number] preserves large integers using math/big rather than silently losing
// precision. [AttributeError] reports unsupported attribute or number forms.
//
// # Module boundary
//
// This optional SDK adapter delegates conversion policy to the dependency-free
// Commons package. [UnmarshallDynamoDB] accepts JSON-shaped wire attributes when
// SDK types are not available. This package does not fetch items or manage tables;
// use an application-owned SDK client for those operations.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/COMMONS.md
package dynamodb
