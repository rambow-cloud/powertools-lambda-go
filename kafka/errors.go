package kafka

// ConsumerError reports an invalid event or consumer configuration.
type ConsumerError struct {
	Message string
	Cause   error
}

func (e *ConsumerError) Error() string     { return e.Message }
func (e *ConsumerError) Unwrap() error     { return e.Cause }
func (e *ConsumerError) ErrorName() string { return "KafkaConsumerError" }

// As lets errors.As match the common consumer base for embedded error variants.
func (e *ConsumerError) As(target any) bool {
	if base, ok := target.(**ConsumerError); ok {
		*base = e
		return true
	}
	return false
}

// DeserializationError reports an unsupported format or binary codec failure.
type DeserializationError struct{ ConsumerError }

func (e *DeserializationError) ErrorName() string { return "KafkaConsumerDeserializationError" }

// MissingSchemaError is raised on field access, unless that field is absent or null.
type MissingSchemaError struct{ ConsumerError }

func (e *MissingSchemaError) ErrorName() string { return "KafkaConsumerMissingSchemaError" }

// ParserError retains Standard Schema issues, including an empty issue array.
type ParserError struct {
	ConsumerError
	Issues []any
}

func (e *ParserError) ErrorName() string { return "KafkaConsumerParserError" }

// TypeError represents a reference input error outside the consumer error hierarchy.
type TypeError struct{ Message string }

func (e *TypeError) Error() string     { return e.Message }
func (e *TypeError) ErrorName() string { return "TypeError" }
