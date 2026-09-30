// Package jmespath evaluates JSON queries with optional Powertools functions.
package jmespath

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jmespath-community/go-jmespath/pkg/interpreter"
	"github.com/jmespath-community/go-jmespath/pkg/parsing"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// ErrorKind classifies failures without relying on engine error text.
type ErrorKind string

const (
	EmptyExpression ErrorKind = "empty-expression"
	SyntaxError     ErrorKind = "syntax"
	UnknownFunction ErrorKind = "unknown-function"
	InvalidArity    ErrorKind = "invalid-arity"
	InvalidType     ErrorKind = "invalid-type"
	InvalidValue    ErrorKind = "invalid-value"
)

// Error preserves expression context and the original failure through Unwrap.
type Error struct {
	Kind       ErrorKind
	Expression string
	Function   string
	Err        error
}

func (e *Error) Error() string {
	return fmt.Sprintf("jmespath %s in %q: %v", e.Kind, e.Expression, e.Err)
}
func (e *Error) Unwrap() error { return e.Err }

// Expression is immutable and reusable across goroutines. Custom functions must
// be concurrency-safe and must not mutate or retain their input values.
type Expression struct {
	source    string
	node      parsing.ASTNode
	functions functionTable
}

var parsedCache = commons.NewLRUCache[string, parsing.ASTNode](128)

// PurgeCache removes cached syntax trees; already compiled expressions remain valid.
func PurgeCache() { parsedCache.Clear() }

// Compile prepares an expression. Standard functions are enabled by default;
// use WithPowertoolsFunctions for JSON, Base64 and gzip decoding.
func Compile(source string, options ...Option) (*Expression, error) {
	if strings.TrimSpace(source) == "" {
		return nil, &Error{Kind: EmptyExpression, Expression: source, Err: fmt.Errorf("expression must not be empty")}
	}
	node, found := parsedCache.Get(source)
	if !found {
		var err error
		node, err = parsing.NewParser().Parse(source)
		if err != nil {
			return nil, &Error{Kind: SyntaxError, Expression: source, Err: err}
		}
		if err = validateSyntax(node); err != nil {
			return nil, &Error{Kind: SyntaxError, Expression: source, Err: err}
		}
		parsedCache.Add(source, node)
	}
	functions := standardFunctions()
	for _, option := range options {
		if option != nil {
			if err := option(functions); err != nil {
				return nil, &Error{Kind: InvalidValue, Expression: source, Err: err}
			}
		}
	}
	return &Expression{source: source, node: node, functions: functions}, nil
}

func MustCompile(source string, options ...Option) *Expression {
	expression, err := Compile(source, options...)
	if err != nil {
		panic(err)
	}
	return expression
}

func Search(source string, data any, options ...Option) (any, error) {
	expression, err := Compile(source, options...)
	if err != nil {
		return nil, err
	}
	return expression.Search(data)
}

// Search accepts JSON-shaped values and typed Go events with JSON field tags.
// Data is normalized to a private JSON snapshot; numbers use float64 as in JS.
func (e *Expression) Search(data any) (any, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, &Error{Kind: InvalidValue, Expression: e.source, Err: err}
	}
	var input any
	if err = json.Unmarshal(encoded, &input); err != nil {
		return nil, &Error{Kind: InvalidValue, Expression: e.source, Err: err}
	}
	result, err := interpreter.NewInterpreter(input, e.functions, nil).Execute(e.node, input)
	if err != nil {
		var queryError *Error
		if errors.As(err, &queryError) {
			copy := *queryError
			copy.Expression = e.source
			return nil, &copy
		}
		return nil, &Error{Kind: InvalidValue, Expression: e.source, Err: err}
	}
	return commons.CloneValue(result), nil
}

func validateSyntax(node parsing.ASTNode) error {
	switch node.NodeType {
	case parsing.ASTArithmeticExpression, parsing.ASTArithmeticUnaryExpression, parsing.ASTRootNode, parsing.ASTLetExpression, parsing.ASTVariable, parsing.ASTBindings, parsing.ASTBinding:
		return fmt.Errorf("community grammar extensions are outside the pinned JMESPath contract")
	}
	for _, child := range node.Children {
		if err := validateSyntax(child); err != nil {
			return err
		}
	}
	return nil
}
