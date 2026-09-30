package datamasking

import (
	"context"
	"fmt"
	"maps"
)

func (m *Masker) requireProvider() error {
	if m.config.Provider == nil {
		return &Error{"DataMaskingEncryptionError", "Encryption provider is required for encrypt/decrypt operations", nil}
	}
	return nil
}

func (m *Masker) Encrypt(ctx context.Context, data any, options TransformOptions) (any, error) {
	return m.transform(ctx, data, options, false)
}

func (m *Masker) Decrypt(ctx context.Context, data any, options TransformOptions) (any, error) {
	return m.transform(ctx, data, options, true)
}

func (m *Masker) transform(ctx context.Context, data any, options TransformOptions, decrypt bool) (any, error) {
	if err := m.requireProvider(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	copy, err := copyInput(data)
	if err != nil {
		return nil, unsupported(err)
	}
	if text, ok := copy.scalar.(string); decrypt && ok {
		plain, err := m.config.Provider.Decrypt(ctx, text, maps.Clone(options.Context))
		if err != nil {
			return nil, err
		}
		decoded, err := parse([]byte(plain))
		if err != nil {
			return nil, err
		}
		return decoded.value(), nil
	}
	if !decrypt && options.Fields == nil {
		plain, err := copy.stringify()
		if err != nil {
			return nil, err
		}
		return m.config.Provider.Encrypt(ctx, plain, maps.Clone(options.Context))
	}
	type operation struct {
		path  []string
		input string
	}
	operations := []operation{}
	for _, field := range options.Fields {
		for _, path := range resolve(copy, field) {
			value := get(copy, path)
			var text string
			if decrypt {
				var ok bool
				text, ok = value.scalar.(string)
				if !ok {
					m.config.Warn(ctx, fmt.Sprintf("Skipping decryption of non-string value of type %s; expected an encrypted string", value.typeName()))
					continue
				}
			} else {
				text, err = value.stringify()
				if err != nil {
					return nil, err
				}
			}
			operations = append(operations, operation{path, text})
		}
	}
	type completion struct {
		path       []string
		value      *node
		err        error
		panicValue any
	}
	completed := make(chan completion, len(operations))
	for _, operation := range operations {
		go func() {
			result := completion{path: operation.path}
			defer func() {
				result.panicValue = recover()
				completed <- result
			}()
			if err := ctx.Err(); err != nil {
				result.err = err
			} else if decrypt {
				var plaintext string
				plaintext, result.err = m.config.Provider.Decrypt(ctx, operation.input, maps.Clone(options.Context))
				if result.err == nil {
					result.value, result.err = parse([]byte(plaintext))
				}
			} else {
				var ciphertext string
				ciphertext, result.err = m.config.Provider.Encrypt(ctx, operation.input, maps.Clone(options.Context))
				result.value = &node{scalar: ciphertext}
			}
		}()
	}
	// Promise.all rejects on the first failure without canceling sibling calls.
	// The buffered channel lets already-started providers finish after return.
	// Only this caller mutates the private result tree.
	for range operations {
		result := <-completed
		if result.panicValue != nil {
			panic(result.panicValue)
		}
		if result.err != nil {
			return nil, result.err
		}
		if err := set(copy, result.path, result.value); err != nil {
			return nil, err
		}
	}
	return copy.value(), nil
}

func (n *node) typeName() string {
	switch n.scalar.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case Undefined:
		return "undefined"
	default:
		return "object"
	}
}
