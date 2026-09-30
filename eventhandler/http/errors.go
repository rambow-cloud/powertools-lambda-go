package http

import (
	"errors"
	"reflect"
)

// HTTPError represents the reference's named HTTP error classes.
type HTTPError struct {
	StatusCode int    `json:"statusCode"`
	Type       string `json:"error"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"`
	Cause      error  `json:"-"`
}

func (e *HTTPError) Error() string { return e.Message }
func (e *HTTPError) Unwrap() error { return e.Cause }

// NewHTTPError selects the reference error name for its built-in status codes.
// Set Type explicitly for application errors or validation errors.
func NewHTTPError(status int, message string) *HTTPError {
	names := map[int]string{400: "BadRequestError", 401: "UnauthorizedError", 403: "ForbiddenError", 404: "NotFoundError", 405: "MethodNotAllowedError", 408: "RequestTimeoutError", 413: "RequestEntityTooLargeError", 500: "InternalServerError", 503: "ServiceUnavailableError"}
	kind := names[status]
	if kind == "" {
		kind = "HttpError"
	}
	return &HTTPError{StatusCode: status, Type: kind, Message: message}
}

type InvalidEventError struct{}

// RequestConversionError identifies conversion failures before HTTP middleware.
// Its cause retains the adapter's diagnostic without becoming a 500 response.
type RequestConversionError struct{ Cause error }

func (e *RequestConversionError) Error() string { return e.Cause.Error() }
func (e *RequestConversionError) Unwrap() error { return e.Cause }

func conversionError(err error) error {
	if err == nil {
		return nil
	}
	var invalid *InvalidEventError
	var method *InvalidHTTPMethodError
	if errors.As(err, &invalid) || errors.As(err, &method) {
		return err
	}
	return &RequestConversionError{Cause: err}
}

func (*InvalidEventError) Error() string { return "event is not compatible with the HTTP resolver" }

type InvalidHTTPMethodError struct{ Method string }

func (e *InvalidHTTPMethodError) Error() string {
	return "HTTP method " + e.Method + " is not supported."
}

type ParameterValidationError struct{ Message string }

func (e *ParameterValidationError) Error() string { return e.Message }

func errorName(err error) string {
	var failure *HTTPError
	if errors.As(err, &failure) {
		return failure.Type
	}
	t := reflect.TypeOf(err)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Name() == "errorString" || t.Name() == "wrapError" {
		return "Error"
	}
	return t.Name()
}
