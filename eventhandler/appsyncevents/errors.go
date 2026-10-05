package appsyncevents

import "errors"

// NamedError supplies the JavaScript error name used in response envelopes.
type NamedError struct {
	Name, Message string
	Cause         error
}

func (e *NamedError) Error() string     { return e.Message }
func (e *NamedError) Unwrap() error     { return e.Cause }
func (e *NamedError) ErrorName() string { return e.Name }

// UnauthorizedException preserves the Go Lambda SDK's reflected errorType as well
// as the reference response name. All publish and subscribe handlers propagate it.
type UnauthorizedException struct {
	Message string
	Cause   error
}

func (e *UnauthorizedException) Error() string     { return e.Message }
func (e *UnauthorizedException) Unwrap() error     { return e.Cause }
func (e *UnauthorizedException) ErrorName() string { return "UnauthorizedException" }

// UnauthorizedError is the Go-style alias; it retains the runtime exception type name.
type UnauthorizedError = UnauthorizedException

type thrownValue struct{}

func (thrownValue) Error() string { return "An unknown error occurred" }

func call(handler func() (any, error)) (value any, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if typed, ok := failure.(error); ok {
				err = typed
			} else {
				err = thrownValue{}
			}
		}
	}()
	return handler()
}

func errorResponse(err error) map[string]any {
	var unknown thrownValue
	if errors.As(err, &unknown) {
		return map[string]any{"error": "An unknown error occurred"}
	}
	name := "Error"
	var named interface{ ErrorName() string }
	if errors.As(err, &named) {
		name = named.ErrorName()
	}
	return map[string]any{"error": name + " - " + err.Error()}
}
