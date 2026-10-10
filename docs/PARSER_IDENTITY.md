---
description: "Parse AppSync and Cognito Lambda events using Powertools Go models, including identity-event contracts and validation scope."
---

# Parser AppSync and Cognito models

Reference: Powertools TypeScript v2.35.0 with Zod v4.1.12. The implementation adds the final three initial schema families: AppSync/shared, AppSync Events and Cognito. Their 29 unique runtime exports are available from `parser/schemas`; shared identities are reused directly. They add no module or third-party dependency. Parser schemas validate events; they do not provide AppSync routing, authorization decisions or a Cognito authentication implementation.

## AppSync exports

| Go export | Contract |
| --- | --- |
| `AppSyncIamIdentity` | Required IAM fields, nullable Cognito identity fields, unrestricted source-IP strings |
| `AppSyncCognitoIdentity` | Required claims dictionary, IPv4 source addresses, nullable groups and default strategy |
| `AppSyncOidcIdentity` | Issuer/sub strings and arbitrary claims; absent claims are accepted |
| `AppSyncLambdaIdentity` | Arbitrary resolver context; absence is accepted |
| `AppSyncResolverSchema` | Arguments/source/request/info/previous result/stash and optional identity |
| `AppSyncBatchResolverSchema` | Resolver array, including an empty array |
| `AppSyncLambdaAuthIdentity` | Required handler-context dictionary for Events Lambda authorization |
| `AppSyncEventsRequestSchema` | Optional headers and required nullable domain name |
| `AppSyncEventsInfoSchema` | Channel path/segments, namespace and PUBLISH/SUBSCRIBE operation |
| `AppSyncEventsBaseSchema` | Shared identity, null fields, request/info, stripped stash and retained output errors |
| `AppSyncEventsPublishSchema` | PUBLISH operation and a nonempty array of id/payload records |
| `AppSyncEventsSubscribeSchema` | SUBSCRIBE operation and explicit null events |

Resolver identity branches are tried in reference order: Cognito, IAM, OIDC, Lambda. The Lambda resolver-context schema accepts an absent context, so an otherwise unrecognized identity object can become an empty object. A matching earlier branch strips fields belonging only to later branches. Events use a different union: null, Cognito, IAM, Lambda handler context, OIDC. These schema rules must not be used as proof that a request is authorized.

Resolver `source` and `prev` are required nullable fields. Its request headers are required, unlike Events headers. Events require explicit null for result/error/prev; publish replaces the base events field with a nonempty collection, while subscribe keeps it null. `parser.Null()` preserves the distinction between explicit null and an absent property.

Extend the arguments schema to validate application data before a typed handler:

```go
schema := schemas.AppSyncResolverSchema.Extend(parser.Field{
    Name: "arguments",
    Schema: parser.Object(parser.Field{Name: "id", Schema: parser.String()}),
})
```

See [the AppSync Lambda example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/parser/appsync/main.go). It combines schema extension, `Typed` and `WrapHandler` without importing a router. Schema extension leaves the exported base model unchanged.

## Cognito exports

| Go exports | Contract |
| --- | --- |
| `CognitoTriggerBaseSchema` | Shared headers, required nullable caller client ID, optional user name; request/response strip unknown properties |
| `PreSignupTriggerSchema` | Sign-up/admin-create/external-provider sources, required nullable validation data and three literal-false response flags |
| `PostConfirmationTriggerSchema` | Confirm-sign-up/forgot-password sources and attributes/nullish client metadata |
| `PreAuthenticationTriggerSchema`, `PostAuthenticationTriggerSchema` | Fixed authentication sources and their distinct request fields |
| `PreTokenGenerationTriggerGroupConfigurationSchema`, `PreTokenGenerationTriggerRequestSchema` | Shared groups, roles, attributes and client metadata |
| `PreTokenGenerationTriggerSchemaV1`, `PreTokenGenerationTriggerSchemaV2AndV3` | Token request variants; only V2/V3 retains optional scopes |
| `MigrateUserTriggerSchema` | Required user name/password and required nullable migration response fields |
| `CustomMessageTriggerSchema` | Code/link/username parameters and nullable custom message response fields |
| `CustomEmailSenderTriggerSchema`, `CustomSMSSenderTriggerSchema` | Documented sender sources, matching request type/code/attributes and optional nullable response |
| `ChallengeResultSchema` | Nine literal challenge names, result boolean and optional metadata |
| `DefineAuthChallengeTriggerSchema`, `CreateAuthChallengeTriggerSchema` | Fixed sources, sessions including the initial empty array and nullish response fields |
| `VerifyAuthChallengeTriggerSchema` | Fixed source, challenge answer/private parameters and required nullable answer-correct boolean |

The Go models intentionally correct the pinned schema limits for [documented trigger sources](https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-user-pools-working-with-lambda-triggers.html): three PreSignup sources, two PostConfirmation sources, eight [email sender](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-lambda-custom-email-sender.html) sources and seven [SMS sender](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-lambda-custom-sms-sender.html) sources. Email alone supports AccountTakeOverNotification. Client metadata accepts absence/null but validates string-valued dictionaries when present. Sender response accepts absence/null and retains the existing supplied-object stripping behavior; other trigger responses remain required.

The [upstream captured-event report](https://github.com/aws-powertools/powertools-lambda-typescript/issues/5770) also demonstrates nullable caller client IDs, initial empty challenge sessions and an unset/null answerCorrect before the handler fills it. Missing required clientId/answerCorrect and wrong present types still fail. These are synthetic Go regression cases, not live service acceptance. MigrateUser, CustomMessage and token-generation schemas retain an unrestricted trigger-source string; token schema names do not enforce the version string.

PreSignup input response flags must be false. Input validation runs before business code and does not revalidate the returned response. A handler can change those flags according to its application policy. [The Cognito example](https://github.com/rambow-cloud/powertools-lambda-go/blob/main/examples/parser/cognito/main.go) validates raw JSON into the native AWS Go event type and applies a required-email rule while preserving the default confirmation flags.

Prefer `json.RawMessage` input when absence matters: marshaling a native AWS Go event struct can emit null for optional maps, turning an absent accepted property into an explicitly null rejected property. Use `Typed` after validation for a native handler value. Go destination structs may omit fields they do not declare; that conversion is an explicit application choice.

## Evidence and remaining work

`generate-parser-identity.mjs` executes every unique export and checks that no public export is missing a sample. Its 1,467 cases cover valid/empty/null inputs, recursive field omission/null/wrong-type mutations, identity selection and stripping, all challenge names, nullable/optional distinctions, empty sessions, publish/subscribe rules, fixed trigger-source limits and false-only flags. The six Null cases verify primitive behavior. This milestone brought Parser to 2,341 reference cases across five corpora; current recursive-error coverage is recorded in PARSER_ERRORS.md.

The generator also maps all 90 unique public runtime schema names, including wildcard subpaths and re-exports, to named Go definitions in [PARSER_SCHEMA_MAP.json](PARSER_SCHEMA_MAP.json). This is a runtime-name mapping, not a proof of inferred-type equivalence. All 24 initial schema families and fourteen envelope families have implementations. Complete declaration/type mapping, full Zod error trees and union semantics, exhaustive numeric/encoding behavior, performance budgets, Validation and Event Handler integration remain open.

Acceptance on 2026-09-15 passed all 18 independently packaged modules and 15 standalone consumers, both CGO-disabled Linux builds, 274/274 Docker assertions and 14/14 Batch checks over the same artifacts. The 24 added assertions cover typed resolver arguments, resolver batches, publish snapshots, subscribe nulls, native Cognito responses, input rejection before business code, token scopes and empty challenge sessions. The runtime executed amd64; arm64 was cross-compiled only. Containers and network were cleaned without AWS access. See [PARSER_PLAN.md](PARSER_PLAN.md) and [LOCAL_VALIDATION.md](LOCAL_VALIDATION.md). Local fixtures do not establish live AppSync or Cognito service acceptance.
