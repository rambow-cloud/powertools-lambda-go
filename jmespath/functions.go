package jmespath

import (
	"fmt"
	"strings"

	engine "github.com/jmespath-community/go-jmespath/pkg/functions"
)

type Type = engine.JpType
type ExpressionReference = engine.ExpRef

const (
	Number      Type = engine.JpNumber
	String      Type = engine.JpString
	Array       Type = engine.JpArray
	Object      Type = engine.JpObject
	ArrayNumber Type = engine.JpArrayNumber
	ArrayString Type = engine.JpArrayString
	Expref      Type = engine.JpExpref
	Any         Type = engine.JpAny
	Boolean     Type = "boolean"
	Null        Type = "null"
)

// Argument accepts any listed type. Only the final argument may be variadic.
type Argument struct {
	Types    []Type
	Variadic bool
}
type Function struct {
	Name      string
	Arguments []Argument
	Handler   func([]any) (any, error)
}
type functionTable map[string]Function
type Option func(functionTable) error

// WithFunctions snapshots definitions and allows overriding built-in functions.
func WithFunctions(functions ...Function) Option {
	copies := make([]Function, len(functions))
	for i, function := range functions {
		copies[i] = function
		copies[i].Arguments = append([]Argument(nil), function.Arguments...)
		for j := range copies[i].Arguments {
			copies[i].Arguments[j].Types = append([]Type(nil), function.Arguments[j].Types...)
		}
	}
	return func(table functionTable) error {
		for _, function := range copies {
			if function.Name == "" || function.Handler == nil {
				return fmt.Errorf("custom functions require a name and handler")
			}
			for i, argument := range function.Arguments {
				if len(argument.Types) == 0 || (argument.Variadic && i != len(function.Arguments)-1) {
					return fmt.Errorf("invalid signature for %s", function.Name)
				}
				for _, kind := range argument.Types {
					switch kind {
					case Number, String, Array, Object, ArrayNumber, ArrayString, Expref, Any, Boolean, Null:
					default:
						return fmt.Errorf("unknown argument type %q", kind)
					}
				}
			}
			table[function.Name] = function
		}
		return nil
	}
}

func standardFunctions() functionTable {
	// Keep community-only functions opt-in through user definitions.
	names := " abs avg ceil contains ends_with floor join keys length map max max_by merge min min_by not_null reverse sort sort_by starts_with sum to_array to_number to_string type values "
	table := functionTable{}
	for _, entry := range engine.GetDefaultFunctions() {
		if !strings.Contains(names, " "+entry.Name+" ") {
			continue
		}
		function := Function{Name: entry.Name, Handler: entry.Handler}
		for _, arg := range entry.Arguments {
			function.Arguments = append(function.Arguments, Argument{Types: arg.Types, Variadic: arg.Variadic})
		}
		if entry.Name == "sort_by" {
			handler := function.Handler
			function.Handler = func(args []any) (any, error) {
				copy := append([]any(nil), args...)
				copy[0] = append([]any{}, args[0].([]any)...)
				return handler(copy)
			}
		}
		table[entry.Name] = function
	}
	return table
}

func (table functionTable) CallFunction(name string, arguments []any) (any, error) {
	function, found := table[name]
	if !found {
		return nil, &Error{Kind: UnknownFunction, Function: name, Err: fmt.Errorf("unknown function %s", name)}
	}
	count := len(function.Arguments)
	variadic := count > 0 && function.Arguments[count-1].Variadic
	if len(arguments) < count || (!variadic && len(arguments) != count) {
		return nil, &Error{Kind: InvalidArity, Function: name, Err: fmt.Errorf("%s expects %d arguments (variadic=%t), received %d", name, count, variadic, len(arguments))}
	}
	for i, value := range arguments {
		index := min(i, count-1)
		matched := false
		for _, kind := range function.Arguments[index].Types {
			matched = matched || matches(kind, value)
		}
		if !matched {
			return nil, &Error{Kind: InvalidType, Function: name, Err: fmt.Errorf("invalid argument %d for %s", i+1, name)}
		}
	}
	return function.Handler(arguments)
}

func matches(kind Type, value any) bool {
	switch kind {
	case Any:
		return true
	case Number:
		_, ok := value.(float64)
		return ok
	case String:
		_, ok := value.(string)
		return ok
	case Object:
		_, ok := value.(map[string]any)
		return ok
	case Boolean:
		_, ok := value.(bool)
		return ok
	case Null:
		return value == nil
	case Expref:
		_, ok := value.(engine.ExpRef)
		return ok
	case Array, ArrayNumber, ArrayString:
		items, ok := value.([]any)
		if !ok {
			return false
		}
		for _, item := range items {
			if kind == ArrayNumber && !matches(Number, item) || kind == ArrayString && !matches(String, item) {
				return false
			}
		}
		return true
	}
	return false
}
