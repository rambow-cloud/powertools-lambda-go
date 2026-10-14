---
description: "Use shared ECMAScript regular expression matching and replacement in Powertools for Go with documented Unicode behavior."
---

# ECMAScript regular expressions

Match and replace text with ECMAScript-style captures and replacement tokens. The optional module is shared by Validation and Data Masking.

## Keep an account prefix

Install `commons/regex@v1.2.0` and import it as `regex`.

**Input**

```text
1234567890
```

**Code**

```go
expression, err := regex.Compile(`(?<prefix>\d{4})\d+`, "g", regex.Options{})
if err != nil { return err }
masked, err := expression.Replace("1234567890", "$<prefix>****")
if err != nil { return err }
fmt.Println(masked)
```

Run this inside a function returning `error`.

**Output**

```text
1234****
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
