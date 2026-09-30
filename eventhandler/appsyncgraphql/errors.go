package appsyncgraphql

import "errors"

// TypeError preserves the reference's empty-batch error name in the Lambda SDK.
type TypeError struct{ Message string }

func (e *TypeError) Error() string     { return e.Message }
func (e *TypeError) ErrorName() string { return "TypeError" }

// NamedError supplies the reference error name without coupling to another utility.
type NamedError struct {
	Name, Message string
	Cause         error
}

func (e *NamedError) Error() string     { return e.Message }
func (e *NamedError) ErrorName() string { return e.Name }
func (e *NamedError) Unwrap() error     { return e.Cause }

// ResolverNotFoundException bypasses registered exception handlers. Its concrete
// type name also preserves errorType in the Go Lambda Runtime API response.
type ResolverNotFoundException struct {
	Message string
	Cause   error
}

func (e *ResolverNotFoundException) Error() string     { return e.Message }
func (e *ResolverNotFoundException) ErrorName() string { return "ResolverNotFoundException" }
func (e *ResolverNotFoundException) Unwrap() error     { return e.Cause }

// InvalidBatchResponseException reports a non-array aggregate result.
type InvalidBatchResponseException struct {
	Message string
	Cause   error
}

func (e *InvalidBatchResponseException) Error() string     { return e.Message }
func (e *InvalidBatchResponseException) ErrorName() string { return "InvalidBatchResponseException" }
func (e *InvalidBatchResponseException) Unwrap() error     { return e.Cause }

type thrownValue struct{}

func (thrownValue) Error() string { return "An unknown error occurred" }

func call(fn func() (any, error)) (value any, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if e, ok := failure.(error); ok {
				err = e
			} else {
				err = thrownValue{}
			}
		}
	}()
	return fn()
}

func errorName(err error) string {
	var named interface{ ErrorName() string }
	if errors.As(err, &named) {
		return named.ErrorName()
	}
	return "Error"
}
