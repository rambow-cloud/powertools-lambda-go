# Logger stacks (Preview)

This Preview is available only from main and is not included in v1.2.0.

From a checkout of `main`, run with the repository workspace:

```sh
CGO_ENABLED=0 go run ./examples/logger-stack-preview
```

For an independent application, copy `main.go` and install the Preview explicitly:

```sh
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@main
CGO_ENABLED=0 go run .
```

The first ERROR has `error.stack_source: "attached"` and a location in `loadOrder`.
The second has `error.stack_source: "logging"` and a location at the second
`appLog.Error` call. Both use one compact JSON line with escaped stack newlines.
Paths and line numbers depend on the checkout. A logging-site stack does not
identify the original error location. Neither example needs a business panic.

See the [Preview guide](../../docs/LOGGER.md#error-stacks-and-panic-logging-preview)
for output, handler panic logging, and opt-out settings. A branch query resolves
to a Go pseudo-version; pin a commit SHA for repeatable previews.
