package datamasking

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode/utf16"
)

const DefaultMask = "*****"

// Undefined preserves missing JavaScript values separately from null.
type Undefined struct{}

// Provider owns ciphertext format, cryptography and authenticated context.
// Implementations must support concurrent calls and honor cancellation.
type Provider interface {
	Encrypt(context.Context, string, map[string]string) (string, error)
	Decrypt(context.Context, string, map[string]string) (string, error)
}

type Config struct {
	Provider      Provider
	IgnoreMissing bool
	Warn          func(context.Context, string)
}

// Rule uses pointers to retain explicit false and empty custom masks.
// Replace is an application-owned string replacement strategy, with precedence
// over CustomMask and DynamicMask. The optional commons/regex module provides
// Regexp.Replacer for ECMAScript string replacement without a core dependency.
type Rule struct {
	CustomMask  *string
	DynamicMask *bool
	Replace     func(string) (string, error)
}

// FieldRule order is significant when expressions overlap.
type FieldRule struct {
	Field string
	Rule  Rule
}

type EraseOptions struct {
	Fields []string
	Rules  []FieldRule
	Rule   Rule
}

type TransformOptions struct {
	Fields  []string
	Context map[string]string
}

type Masker struct{ config Config }

func New(config Config) *Masker {
	if config.Warn == nil {
		config.Warn = func(_ context.Context, message string) { fmt.Fprintln(os.Stderr, message) }
	}
	return &Masker{config: config}
}

type Error struct {
	Name    string
	Message string
	Cause   error
}

func (e *Error) Error() string     { return e.Message }
func (e *Error) ErrorName() string { return e.Name }
func (e *Error) Unwrap() error     { return e.Cause }

func unsupported(cause error) error {
	return &Error{"DataMaskingUnsupportedTypeError", "Data contains unsupported types for cloning", cause}
}

func (r Rule) configured() bool {
	return r.Replace != nil || r.CustomMask != nil || r.DynamicMask != nil
}

func (r Rule) mask(value *node) (*node, error) {
	if value.scalar == nil && value.object == nil && value.array == nil {
		return value, nil
	}
	if _, missing := value.scalar.(Undefined); missing {
		return value, nil
	}
	text := value.text()
	if r.Replace != nil {
		masked, err := r.Replace(text)
		return &node{scalar: masked}, err
	}
	if r.CustomMask != nil {
		return &node{scalar: *r.CustomMask}, nil
	}
	if r.DynamicMask != nil && *r.DynamicMask {
		return &node{scalar: strings.Repeat("*", len(utf16.Encode([]rune(text))))}, nil
	}
	return &node{scalar: DefaultMask}, nil
}

func (m *Masker) Erase(ctx context.Context, data any, options EraseOptions) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	if _, missing := data.(Undefined); missing {
		return data, nil
	}
	// Whole-payload erasure does not clone or inspect object contents upstream.
	// RawMessage must first be decoded to distinguish null, arrays and objects.
	if options.Fields == nil && options.Rules == nil && !options.Rule.configured() {
		if _, raw := data.(json.RawMessage); !raw {
			value := reflect.ValueOf(data)
			switch value.Kind() {
			case reflect.Pointer, reflect.Map, reflect.Slice:
				if value.IsNil() {
					return nil, nil
				}
			}
			_, binary := data.([]byte)
			if !binary && (value.Kind() == reflect.Array || value.Kind() == reflect.Slice) {
				result := make([]any, value.Len())
				for i := range result {
					result[i] = DefaultMask
				}
				return result, nil
			}
			return DefaultMask, nil
		}
	}
	copy, err := copyInput(data)
	if err != nil {
		return nil, unsupported(err)
	}
	if copy.isNull() {
		return nil, nil
	}
	if options.Fields == nil && options.Rules == nil {
		if !options.Rule.configured() {
			if copy.array != nil {
				result := make([]any, len(copy.array))
				for i := range result {
					result[i] = DefaultMask
				}
				return result, nil
			}
			return DefaultMask, nil
		}
		if copy.object == nil && copy.array == nil {
			copy, err = options.Rule.mask(copy)
		} else {
			err = maskLeaves(ctx, copy, options.Rule)
		}
		if err != nil {
			return nil, err
		}
		return copy.value(), nil
	}
	touched := map[string]bool{}
	for _, item := range options.Rules {
		paths := resolve(copy, item.Field)
		if len(paths) == 0 {
			if err := m.missingField(ctx, item.Field); err != nil {
				return nil, err
			}
		}
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			value, err := item.Rule.mask(get(copy, path))
			if err != nil {
				return nil, err
			}
			if err := set(copy, path, value); err != nil {
				return nil, err
			}
			touched[pathKey(path)] = true
		}
	}
	for _, field := range options.Fields {
		paths := resolve(copy, field)
		if len(paths) == 0 {
			if err := m.missingField(ctx, field); err != nil {
				return nil, err
			}
		}
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if touched[pathKey(path)] {
				continue
			}
			value := &node{scalar: DefaultMask}
			if options.Rule.configured() {
				value, err = options.Rule.mask(get(copy, path))
				if err != nil {
					return nil, err
				}
			}
			if err := set(copy, path, value); err != nil {
				return nil, err
			}
		}
	}
	return copy.value(), nil
}

func (m *Masker) missingField(ctx context.Context, field string) error {
	message := fmt.Sprintf("Field not found: '%s'", field)
	if !m.config.IgnoreMissing {
		return &Error{"DataMaskingFieldNotFoundError", message, nil}
	}
	m.config.Warn(ctx, message)
	return nil
}

func maskLeaves(ctx context.Context, value *node, rule Rule) error {
	for _, key := range value.keys() {
		if err := ctx.Err(); err != nil {
			return err
		}
		child := value.child(key)
		if child.object != nil || child.array != nil {
			if err := maskLeaves(ctx, child, rule); err != nil {
				return err
			}
		} else {
			masked, err := rule.mask(child)
			if err != nil {
				return err
			}
			if err := set(value, []string{key}, masked); err != nil {
				return err
			}
		}
	}
	return nil
}
