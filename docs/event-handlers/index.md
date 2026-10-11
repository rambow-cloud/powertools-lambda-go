---
title: "Go Lambda event handlers and HTTP routing"
description: "Build Go Lambda HTTP APIs with Get and Post routing, or resolve AppSync Events, AppSync GraphQL and Bedrock Agent function requests."
category: true
---

# Event handlers

Choose the event handler that matches the AWS service invoking your Go Lambda function. Each handler has its own module and a complete example in its guide.

## Choose your integration

| Incoming request | Guide |
| --- | --- |
| API Gateway, ALB or Lambda Function URL | [HTTP routing](../HTTP.md) |
| AppSync Events publish or subscribe operations | [AppSync Events](../APPSYNC_EVENTS.md) |
| AppSync GraphQL resolver fields | [AppSync GraphQL](../APPSYNC_GRAPHQL.md) |
| Bedrock Agent function-based action groups | [Bedrock Agents](../BEDROCK.md) |

## Build an HTTP API

Start with the [complete GET/POST example](../HTTP.md#complete-example). Register routes with `app.Get` and `app.Post`, read path parameters or JSON bodies, and return a Go value for a JSON response.

Add [middleware](../HTTP_MIDDLEWARE.md) for shared request behavior, then [observability](../HTTP_OBSERVABILITY.md) for logging, metrics and tracing. Read [response streaming](../HTTP_STREAMING.md) when your supported invocation path needs streamed output; check its adapter and runtime boundaries before deploying.

For record batches rather than request routing, use [Data processing](../data-processing/index.md). Installation and runtime prerequisites are in [Getting started](../getting-started/index.md).
