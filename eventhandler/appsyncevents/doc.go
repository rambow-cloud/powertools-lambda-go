// Package appsyncevents routes AWS Lambda AppSync Events invocations.
//
// [New] constructs a [Resolver]. Register channel handlers with
// [Router.OnPublish] and [Router.OnSubscribe], then call [Resolver.Resolve] from
// the Lambda handler. [PublishOptions] selects individual or aggregate publication.
//
// # Concurrency and errors
//
// Individual publication executes handlers concurrently and preserves input order
// in the response. Handlers share the original [Event] envelope and must treat it
// as read-only. [Undefined] omits a payload rather than encoding JSON null.
//
// Subscription authorization and publication error behavior follow the documented
// resolver contract. Invalid events return nil after diagnostics. The application
// owns AppSync namespace setup, authorization policy and diagnostic callbacks.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/APPSYNC_EVENTS.md
package appsyncevents
