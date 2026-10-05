package http

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	nethttp "net/http"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// ValidationIssue retains the Standard Schema issue fields used by HTTP errors.
type ValidationIssue struct {
	Message string `json:"message"`
	Path    []any  `json:"path,omitempty"`
}

// Check adapts Parser, Validation or application schemas without a module dependency.
// Nonempty issues reject a value; operational errors remain distinct.
type Check func(context.Context, any) (any, []ValidationIssue, error)
type RequestChecks struct{ Body, Headers, Path, Query Check }
type ResponseChecks struct{ Body, Headers Check }
type ValidationConfig struct {
	Request  *RequestChecks
	Response *ResponseChecks
}
type ValidatedData struct{ Request, Response map[string]any }

func validationError(inbound bool, body bool, issues []ValidationIssue) *HTTPError {
	status, name, side := 422, "RequestValidationError", "request"
	if !inbound {
		status, name, side = 500, "ResponseValidationError", "response"
	}
	message := "Validation failed for " + side
	if body {
		message += " body"
	}
	return &HTTPError{StatusCode: status, Type: name, Message: message, Details: map[string]any{"issues": issues}}
}
func headerFields(headers nethttp.Header) map[string]any {
	result := map[string]any{}
	for name, values := range headers {
		result[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return result
}
func extractValidationBody(body io.ReadCloser, headers nethttp.Header, inbound bool) (any, error) {
	data, err := readOwnedBody(body)
	if err != nil {
		return nil, err
	}
	text := strings.TrimPrefix(commons.DecodeUTF8(data), "\ufeff")
	media, _, mediaErr := mime.ParseMediaType(headers.Get("Content-Type"))
	_, subtype, _ := strings.Cut(media, "/")
	if mediaErr == nil && (media == "application/json" || len(subtype) > len("+json") && strings.HasSuffix(subtype, "+json")) {
		var result any
		if json.Unmarshal([]byte(text), &result) != nil {
			return nil, validationError(inbound, true, []ValidationIssue{})
		}
		return result, nil
	}
	return text, nil
}

type validationField struct {
	name  string
	check Check
	value any
}

func validateFields(ctx context.Context, fields []validationField, inbound bool) (map[string]any, error) {
	values := map[string]any{}
	var issues []ValidationIssue
	for _, field := range fields {
		if field.check == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, failures, err := field.check(ctx, field.value)
		if err != nil {
			return nil, err
		}
		for _, issue := range failures {
			issue.Path = append([]any{field.name}, issue.Path...)
			issues = append(issues, issue)
		}
		values[field.name] = value
	}
	if len(issues) > 0 {
		return nil, validationError(inbound, false, issues)
	}
	return values, nil
}

// Validate runs request checks before next and response checks after it.
// Transformed values live in Valid; the request and response are not rewritten.
func Validate(config ValidationConfig) Middleware {
	// Snapshot the small configuration; callbacks remain application-owned.
	if config.Request != nil {
		copy := *config.Request
		config.Request = &copy
	}
	if config.Response != nil {
		copy := *config.Response
		config.Response = &copy
	}
	return func(request *RequestContext, next Next) error {
		if checks := config.Request; checks != nil {
			request.Valid.Request = map[string]any{}
			var body any
			if checks.Body != nil {
				var reader io.ReadCloser
				var err error
				if request.Request.GetBody != nil {
					reader, err = request.Request.GetBody()
				} else {
					reader = nethttp.NoBody
				}
				if err != nil {
					return err
				}
				body, err = extractValidationBody(reader, request.Request.Header, true)
				if err != nil {
					return err
				}
			}
			query := map[string]any{}
			for key, values := range request.Request.URL.Query() {
				if len(values) > 0 {
					query[key] = values[len(values)-1]
				}
			}
			path := map[string]any{}
			for key, value := range request.Params {
				path[key] = value
			}
			values, err := validateFields(request.Context, []validationField{{"body", checks.Body, body}, {"headers", checks.Headers, headerFields(request.Request.Header)}, {"path", checks.Path, path}, {"query", checks.Query, query}}, true)
			if err != nil {
				return err
			}
			request.Valid.Request = values
		}
		if config.Response != nil {
			request.Valid.Response = map[string]any{}
		}
		if err := next(); err != nil {
			return err
		}
		if checks := config.Response; checks != nil {
			var body any
			checkBody := checks.Body
			if request.Response.Body == nil || request.Response.Body == nethttp.NoBody {
				checkBody = nil
			}
			if checkBody != nil {
				source := request.Response.Body
				request.Response.Body = nethttp.NoBody
				data, err := readOwnedBody(source)
				if err != nil {
					return err
				}
				request.Response.Body = ioBody(data)
				body, err = extractValidationBody(ioBody(data), request.Response.Header, false)
				if err != nil {
					return err
				}
			}
			values, err := validateFields(request.Context, []validationField{{"body", checkBody, body}, {"headers", checks.Headers, headerFields(request.Response.Header)}}, false)
			if err != nil {
				return err
			}
			request.Valid.Response = values
		}
		return nil
	}
}
