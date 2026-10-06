// Package schemas supplies reusable AWS Lambda service event schemas.
//
// Exports such as [SqsSchema], [EventBridgeSchema] and [APIGatewayProxyEventSchema]
// are Parser schemas for validating decoded events. They preserve each family's
// documented required fields, optional fields and compatibility aliases.
//
// # Application payloads
//
// These contracts validate the service envelope rather than every application's
// payload. Compose the parser/envelopes package with a payload schema to validate
// message bodies or selected detail fields. The guide maps supported service
// families and exported schema names and records known compatibility boundaries.
//
// Importing this package does not make AWS requests or initialize service clients.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/PARSER.md
package schemas
