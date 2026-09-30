package validation

type Issue struct {
	InstancePath string         `json:"instancePath"`
	SchemaPath   string         `json:"schemaPath"`
	Keyword      string         `json:"keyword"`
	Message      string         `json:"message"`
	Params       map[string]any `json:"params"`
	PropertyName *string        `json:"propertyName,omitempty"`
}

type SchemaCompilationError struct{ Err error }

func (e *SchemaCompilationError) Error() string { return "Failed to compile schema" }
func (e *SchemaCompilationError) Unwrap() error { return e.Err }

type SchemaValidationError struct {
	Message string
	Issues  []Issue
	Err     error
}

func (e *SchemaValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "Schema validation failed"
}
func (e *SchemaValidationError) Unwrap() error { return e.Err }
