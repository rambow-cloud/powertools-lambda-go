package bedrock

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type Event = map[string]any
type ToolHandler func(context.Context, *Parameters, Event) (any, error)
type Configuration struct{ Name, Description string }
type Options struct {
	// Diagnostic runs outside locks and may be called concurrently. Registration
	// uses context.Background; execution diagnostics retain the invocation context.
	Diagnostic func(context.Context, string, string, error)
}

// Error preserves the ordinary JavaScript Error name in Lambda Runtime API errors.
type Error struct {
	Message string
	Cause   error
}

func (e *Error) Error() string     { return e.Message }
func (e *Error) ErrorName() string { return "Error" }
func (e *Error) Unwrap() error     { return e.Cause }

type NamedError struct {
	Name, Message string
	Cause         error
}

func (e *NamedError) Error() string     { return e.Message }
func (e *NamedError) ErrorName() string { return e.Name }
func (e *NamedError) Unwrap() error     { return e.Cause }

type thrownValue struct{ value any }

func (e thrownValue) Error() string { return thrownString(e.value) }

type Resolver struct {
	mu      sync.RWMutex
	tools   map[string]ToolHandler
	options Options
	debug   bool
}

func New(options Options) *Resolver {
	level, _ := commons.StringEnv("AWS_LAMBDA_LOG_LEVEL", "")
	return &Resolver{tools: map[string]ToolHandler{}, options: options, debug: level == "DEBUG"}
}

func (r *Resolver) diagnostic(ctx context.Context, level, message string, err error) {
	if r.options.Diagnostic != nil {
		r.options.Diagnostic(ctx, level, message, err)
		return
	}
	if level == "debug" && !r.debug {
		return
	}
	output := os.Stderr
	if level == "debug" {
		output = os.Stdout
	}
	if err == nil {
		fmt.Fprintln(output, message)
	} else {
		fmt.Fprintln(output, message, err)
	}
}

// Tool registers a handler by function name. Description is accepted for source
// compatibility; the reference stores it without validating or publishing it.
func (r *Resolver) Tool(handler ToolHandler, configuration Configuration) {
	ctx := context.Background()
	r.mu.RLock()
	_, exists := r.tools[configuration.Name]
	r.mu.RUnlock()
	if exists {
		r.diagnostic(ctx, "warn", "Tool \""+configuration.Name+"\" already registered. Overwriting with new definition.", nil)
	}
	r.mu.Lock()
	r.tools[configuration.Name] = handler
	r.mu.Unlock()
	r.diagnostic(ctx, "debug", "Tool \""+configuration.Name+"\" has been registered.", nil)
}

// Resolve validates the event before calling a tool. Business and serialization
// failures become Bedrock response bodies; malformed input returns an error.
func (r *Resolver) Resolve(ctx context.Context, input any) (any, error) {
	event, valid := validEvent(input)
	if !valid {
		return nil, &Error{Message: "Event is not a valid BedrockAgentFunctionEvent"}
	}
	name, group := event["function"].(string), event["actionGroup"].(string)
	// Capture these references before invoking a tool, as the reference does.
	session, prompt, knowledge := event["sessionAttributes"], event["promptSessionAttributes"], event["knowledgeBasesConfiguration"]
	build := func(body any) map[string]any {
		return (&FunctionResponse{Body: body, SessionAttributes: session, PromptSessionAttributes: prompt, KnowledgeBasesConfiguration: knowledge}).Build(group, name)
	}
	r.mu.RLock()
	handler, exists := r.tools[name]
	r.mu.RUnlock()
	if !exists {
		r.diagnostic(ctx, "error", "Tool \""+name+"\" has not been registered.", nil)
		return build("Error: tool \"" + name + "\" has not been registered."), nil
	}
	parameters := &Parameters{}
	items, _ := event["parameters"].([]any)
	for _, item := range items {
		parameter := item.(map[string]any)
		key, kind, raw := parameter["name"].(string), parameter["type"].(string), parameter["value"].(string)
		// JavaScript's inherited __proto__ setter ignores these primitive values.
		if key == "__proto__" {
			continue
		}
		var value any = raw
		switch kind {
		case "boolean":
			value = raw == "true"
		case "number", "integer":
			if number := commons.ParseNumber(raw); !math.IsNaN(number) {
				value = number
			}
		}
		parameters.Set(key, value)
	}
	value, err := call(func() (any, error) {
		value, err := handler(ctx, parameters, event)
		if err != nil {
			return nil, err
		}
		switch explicit := value.(type) {
		case *FunctionResponse:
			if explicit != nil {
				return explicit.Build(group, name), nil
			}
		case FunctionResponse:
			return explicit.Build(group, name), nil
		}
		if nullish(value) {
			return build(""), nil
		}
		if text, ok := value.(string); ok && text == "" {
			return build(""), nil
		}
		body, err := stringify(value)
		if err != nil {
			return nil, err
		}
		return build(string(body)), nil
	})
	if err == nil {
		return value, nil
	}
	r.diagnostic(ctx, "error", "An error occurred in tool "+name+".", err)
	return build("Unable to complete tool execution due to " + errorText(err)), nil
}

func validEvent(input any) (Event, bool) {
	event, ok := input.(map[string]any)
	if !ok || event == nil {
		return nil, false
	}
	for _, key := range []string{"actionGroup", "function", "messageVersion", "inputText", "sessionId"} {
		if _, ok := event[key].(string); !ok {
			return nil, false
		}
	}
	for _, key := range []string{"agent", "sessionAttributes", "promptSessionAttributes"} {
		if value, ok := event[key].(map[string]any); !ok || value == nil {
			return nil, false
		}
	}
	agent := event["agent"].(map[string]any)
	for _, key := range []string{"name", "id", "alias", "version"} {
		if _, ok := agent[key].(string); !ok {
			return nil, false
		}
	}
	if raw, present := event["parameters"]; present {
		items, ok := raw.([]any)
		if !ok || items == nil {
			return nil, false
		}
		for _, value := range items {
			item, ok := value.(map[string]any)
			if !ok {
				return nil, false
			}
			for _, key := range []string{"name", "type", "value"} {
				if _, ok := item[key].(string); !ok {
					return nil, false
				}
			}
		}
	}
	return event, true
}

func call(fn func() (any, error)) (value any, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if typed, ok := failure.(error); ok {
				err = typed
			} else {
				err = thrownValue{failure}
			}
		}
	}()
	return fn()
}
func errorText(err error) string {
	var thrown thrownValue
	if errors.As(err, &thrown) {
		return thrown.Error()
	}
	name := "Error"
	var named interface{ ErrorName() string }
	if errors.As(err, &named) {
		name = named.ErrorName()
	}
	return name + " - " + err.Error()
}
