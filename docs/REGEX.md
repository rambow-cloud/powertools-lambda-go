---
description: "Use shared ECMAScript regular expression matching and replacement in Powertools for Go with documented Unicode behavior."
---

# ECMAScript regular expressions

Match and replace text with ECMAScript-style captures and replacement tokens. The optional module is shared by Validation and Data Masking.

## Keep an account prefix

Install `commons/regex@v1.3.1` and `logger@v1.3.1`, importing them as `regex` and `logger`.

**Input**

```text
1234567890
```

**Code**

```go
appLog := logger.New(logger.WithServiceName("orders"))
expression, err := regex.Compile(`(?<prefix>\d{4})\d+`, "g", regex.Options{})
if err != nil { return err }
masked, err := expression.Replace("1234567890", "$<prefix>****")
if err != nil { return err }
return appLog.Info("Account masked", logger.Fields{"account": masked})
```

Run this inside a function returning `error`. The Logger timestamp varies on each run.

**Output**

```json
{
  "level": "INFO",
  "message": "Account masked",
  "timestamp": "2026-10-15T00:00:00.000Z",
  "service": "orders",
  "sampling_rate": 0,
  "account": "1234****"
}
```

## Use a replacement in Data Masking

```go
masked, err := datamasking.New(datamasking.Config{}).Erase(ctx, payload,
    datamasking.EraseOptions{
        Fields: []string{"account"},
        Rule: datamasking.Rule{Replace: expression.Replacer("$<prefix>****")},
    })
```

For `{"account":"1234567890"}`, the result is `{"account":"1234****"}`.

## Things to know

- Supported flags are `d`, `g`, `i`, `m`, `s`, `u` and `y`. Unicode sets (`v`) are not supported.
- `MatchString` is a stateless search. Stateful replacement can use `LastIndex` / `SetLastIndex`.
- Separate expressions when different callers need independent state.
- Configure time/backtracking limits for untrusted expressions; resource failures return errors.

## Next steps

[API and behavior notes](reference/REGEX.md) · [HTTP](HTTP.md) · [Modules](MODULES.md)
