# Logger stacks (Preview)

This Preview is included in v1.3.0 and main. It is not included in v1.2.0.

Record an error with its call stack using the Powertools Logger. This example
shows two ways to capture a stack:

- **At the source:** `logger.WithStack(err)` captures where the error is wrapped.
- **When logging:** Logger adds a stack at the `Error` call if the error has no
  attached stack.

The program uses two local errors as input. It needs Go 1.27.3 or later, no AWS
credentials, and no running Lambda function.

## Run from this repository

Check out `main`, then run from the **repository root** with its `go.work` enabled:

```sh
CGO_ENABLED=0 go run ./examples/logger-stack-preview
```

In PowerShell:

```powershell
$env:CGO_ENABLED = "0"
go run ./examples/logger-stack-preview
```

The repository workspace selects the local Preview code. Running this example
with `GOWORK=off` selects the v1.3.0 dependencies declared in `examples/go.mod`,
which also include the Preview. The v1.2.0 cohort does not provide `logger.WithStack`.

## Run in your own application

Create a module and select the published v1.3.0 Preview:

```sh
mkdir logger-stack-demo
cd logger-stack-demo
CGO_ENABLED=0 go mod init example.com/logger-stack-demo
CGO_ENABLED=0 go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

In PowerShell, use:

```powershell
$env:CGO_ENABLED = "0"
mkdir logger-stack-demo
cd logger-stack-demo
go mod init example.com/logger-stack-demo
go get github.com/rambow-cloud/powertools-lambda-go/logger@v1.3.0
```

Save the following complete program as `main.go`. It is the same code as
[`main.go`](main.go) in this directory.

```go
// This Preview is not included in v1.2.0.
package main

import (
	"errors"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func loadOrder() error {
	return logger.WithStack(errors.New("order not found"))
}

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if logErr := appLog.Error("Cannot load order", loadOrder()); logErr != nil {
		panic(logErr)
	}
	if logErr := appLog.Error("Payment failed", errors.New("card declined")); logErr != nil {
		panic(logErr)
	}
}
```

Run it:

```sh
CGO_ENABLED=0 go run .
```

In PowerShell, after setting `CGO_ENABLED` above, run `go run .`.

To select development source instead, use `logger@main`. An omitted version query
does not select `main`. The `@main` query resolves to a
Go pseudo-version. To pin a repeatable Preview, replace `main` with the full SHA
of a commit on `main` that includes the feature. This also lets you select the
desired commit if a module proxy still resolves `main` to an older revision.
See [Go version queries](https://go.dev/ref/mod#version-queries).

## Output

The program writes **two compact JSON records to stderr** and exits successfully
when both writes succeed. It writes nothing to stdout. The timestamps below are
fixed and the source paths are shortened to `/app/main.go`; actual timestamps,
paths, and JSON property order can differ.

```json
{"level":"ERROR","message":"Cannot load order","timestamp":"2026-10-15T10:51:54.147Z","service":"orders","sampling_rate":0,"error":{"name":"*errors.errorString","message":"order not found","location":"/app/main.go:11","stack_source":"attached","stack":"main.loadOrder\n\t/app/main.go:11\nmain.main\n\t/app/main.go:16"}}
{"level":"ERROR","message":"Payment failed","timestamp":"2026-10-15T10:51:54.147Z","service":"orders","sampling_rate":0,"error":{"name":"*errors.errorString","message":"card declined","location":"/app/main.go:19","stack":"main.main\n\t/app/main.go:19","stack_source":"logging"}}
```

| Record | `error.stack_source` | What `error.location` tells you |
| --- | --- | --- |
| `Cannot load order` | `attached` | `loadOrder` called `WithStack` at line 11. |
| `Payment failed` | `logging` | `main` called `Error` at line 19. The original error location was not captured. |

`error.stack` contains function names and file locations. Its newlines are
escaped as `\n` inside JSON, so each record occupies one physical line. Pretty
output and custom formatters can produce different layouts.

Neither business error causes a panic. `Error` returns a logging failure, such
as a writer or serialization error; it does not return the business error passed
to it. The `panic(logErr)` branches in this small executable handle those logging
failures only.

## Turn off the Preview behavior

Both stack traces and automatic handler panic logging are enabled by default.
To disable both, replace the `appLog := ...` declaration in the program with:

```go
appLog := logger.New(
	logger.WithServiceName("orders"),
	logger.WithStackTrace(false),
	logger.WithPanicLogging(false),
)
```

The same program still logs both business errors. Its `error` objects now contain
only the existing error fields:

```json
{"name":"*errors.errorString","message":"order not found","location":""}
{"name":"*errors.errorString","message":"card declined","location":""}
```

These are the `error` objects inside the two records; the outer log fields remain.
`WithStackTrace(false)` suppresses attached and automatic stack fields.
`WithPanicLogging(false)` disables automatic handler panic records. The second
option has no visible effect in this program because it does not wrap a handler.

## Use this in a Lambda handler

`WithStack` can wrap an error before it is returned from your application code;
it preserves the message, unwrapping, and `errors.Is` / `errors.As` behavior.
Passing `nil` returns `nil`. Wrapping an error again retains an existing stack
along a single unwrap chain.

For a real Lambda invocation, use Logger's handler wrapper as shown in the
[Preview guide](../../docs/LOGGER.md#error-stacks-and-panic-logging-preview).
The outermost wrapper logs an unhandled handler panic once at ERROR level with
`error.stack_source: "panic"` and the request context, then rethrows the original
panic. Ordinary returned errors must be logged explicitly; they do not need to
be converted to panics to obtain a stack.

This local program demonstrates `attached` and `logging` stacks. It does not
start the Lambda runtime or demonstrate invocation panic handling. Startup code,
other goroutines, and fatal runtime errors are outside the wrapper's scope.
