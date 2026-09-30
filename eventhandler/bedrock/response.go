package bedrock

import (
	"math"
	"reflect"
)

const (
	Failure  = "FAILURE"
	Reprompt = "REPROMPT"
)

// Undefined represents an absent JavaScript value. Nil represents JSON null.
type Undefined struct{}

// FunctionResponse bypasses ordinary JSON body conversion. Construct it with
// NewFunctionResponse to obtain the reference's empty session-attribute defaults.
// Set optional fields to nil to omit them. Fields remain invocation-owned.
type FunctionResponse struct {
	Body                        any
	ResponseState               any
	SessionAttributes           any
	PromptSessionAttributes     any
	KnowledgeBasesConfiguration any
}

func NewFunctionResponse(body any) *FunctionResponse {
	return &FunctionResponse{Body: body, SessionAttributes: map[string]any{}, PromptSessionAttributes: map[string]any{}}
}

// Build creates the wire envelope without modifying or cloning supplied fields.
func (r *FunctionResponse) Build(actionGroup, function string) map[string]any {
	text := map[string]any{}
	if _, absent := r.Body.(Undefined); !absent {
		text["body"] = r.Body
	}
	response := map[string]any{"responseBody": map[string]any{"TEXT": text}}
	if truthy(r.ResponseState) {
		response["responseState"] = r.ResponseState
	}
	result := map[string]any{"messageVersion": "1.0", "response": map[string]any{"actionGroup": actionGroup, "function": function, "functionResponse": response}}
	for key, value := range map[string]any{"sessionAttributes": r.SessionAttributes, "promptSessionAttributes": r.PromptSessionAttributes, "knowledgeBasesConfiguration": r.KnowledgeBasesConfiguration} {
		if truthy(value) {
			result[key] = value
		}
	}
	return result
}

func truthy(value any) bool {
	if nullish(value) {
		return false
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.String:
		return rv.Len() != 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0 && !math.IsNaN(rv.Float())
	case reflect.Map, reflect.Slice, reflect.Pointer, reflect.Interface, reflect.Func:
		return !rv.IsNil()
	default:
		return true
	}
}

func nullish(value any) bool {
	if value == nil {
		return true
	}
	if _, absent := value.(Undefined); absent {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Pointer, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}
