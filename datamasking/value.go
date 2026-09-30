package datamasking

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// The private tree keeps raw JSON insertion order for provider plaintext and
// wildcard traversal. Native Go maps acquire encoding/json's deterministic order.
type node struct {
	scalar any
	object map[string]*node
	order  []string
	array  []*node
}

func copyInput(input any) (*node, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

func parse(data []byte) (*node, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	result, err := readNode(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after JSON value")
	}
	return result, nil
}

func readNode(decoder *json.Decoder) (*node, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	result := &node{}
	switch token {
	case json.Delim('{'):
		result.object = map[string]*node{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key := token.(string)
			value, err := readNode(decoder)
			if err != nil {
				return nil, err
			}
			if _, present := result.object[key]; !present {
				result.order = append(result.order, key)
			}
			result.object[key] = value
		}
		commons.SortObjectKeys(result.order)
		_, err = decoder.Token()
	case json.Delim('['):
		result.array = []*node{}
		for decoder.More() {
			value, err := readNode(decoder)
			if err != nil {
				return nil, err
			}
			result.array = append(result.array, value)
		}
		_, err = decoder.Token()
	default:
		result.scalar = token
		if number, ok := token.(json.Number); ok {
			result.scalar = commons.ParseNumber(string(number))
		}
	}
	return result, err
}

func (n *node) isNull() bool { return n.scalar == nil && n.object == nil && n.array == nil }

func (n *node) value() any {
	if n.object != nil {
		result := make(map[string]any, len(n.object))
		for key, value := range n.object {
			result[key] = value.value()
		}
		return result
	}
	if n.array != nil {
		result := make([]any, len(n.array))
		for i, value := range n.array {
			result[i] = value.value()
		}
		return result
	}
	return n.scalar
}

func (n *node) text() string {
	if n.object != nil {
		return "[object Object]"
	}
	if n.array != nil {
		parts := make([]string, len(n.array))
		for i, value := range n.array {
			if _, missing := value.scalar.(Undefined); !missing && !value.isNull() {
				parts[i] = value.text()
			}
		}
		return strings.Join(parts, ",")
	}
	switch value := n.scalar.(type) {
	case nil:
		return "null"
	case Undefined:
		return "undefined"
	case float64:
		if value == 0 {
			return "0"
		}
		if math.IsInf(value, 1) {
			return "Infinity"
		}
		if math.IsInf(value, -1) {
			return "-Infinity"
		}
		if math.IsNaN(value) {
			return "NaN"
		}
		data, _ := json.Marshal(value)
		return string(data)
	default:
		return fmt.Sprint(value)
	}
}

func quote(value string) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSuffix(buffer.String(), "\n")
}

func (n *node) stringify() (string, error) {
	if n.object != nil {
		parts := []string{}
		for _, key := range n.order {
			value := n.object[key]
			if _, missing := value.scalar.(Undefined); missing {
				continue
			}
			encoded, err := value.stringify()
			if err != nil {
				return "", err
			}
			parts = append(parts, quote(key)+":"+encoded)
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	}
	if n.array != nil {
		parts := make([]string, len(n.array))
		for i, value := range n.array {
			if _, missing := value.scalar.(Undefined); missing {
				parts[i] = "null"
				continue
			}
			text, err := value.stringify()
			if err != nil {
				return "", err
			}
			parts[i] = text
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	}
	switch value := n.scalar.(type) {
	case Undefined:
		return "", &Error{"DataMaskingUnsupportedTypeError", "Undefined cannot be passed to a Go string encryption provider", nil}
	case string:
		return quote(value), nil
	case float64:
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return "null", nil
		}
		return n.text(), nil
	default:
		data, err := json.Marshal(value)
		return string(data), err
	}
}

func reserved(key string) bool {
	return key == "__proto__" || key == "constructor" || key == "prototype"
}

func (n *node) keys() []string {
	keys := []string{}
	for _, key := range n.order {
		if !reserved(key) {
			keys = append(keys, key)
		}
	}
	for i := range n.array {
		keys = append(keys, strconv.Itoa(i))
	}
	return keys
}

func (n *node) child(key string) *node {
	if n.object != nil {
		return n.object[key]
	}
	if n.array != nil {
		if key == "length" {
			return &node{scalar: float64(len(n.array))}
		}
		index, err := strconv.Atoi(key)
		if err == nil && index >= 0 && index < len(n.array) && strconv.Itoa(index) == key {
			return n.array[index]
		}
	}
	return nil
}

func resolve(root *node, expression string) [][]string {
	segments := strings.FieldsFunc(strings.ReplaceAll(expression, "[*]", ".*."), func(r rune) bool { return r == '.' })
	paths := [][]string{}
	var walk func(*node, int, []string)
	walk = func(current *node, index int, path []string) {
		if index == len(segments) {
			paths = append(paths, append([]string{}, path...))
			return
		}
		keys := []string{segments[index]}
		if segments[index] == "*" {
			keys = current.keys()
		}
		for _, key := range keys {
			if child := current.child(key); child != nil {
				walk(child, index+1, append(path, key))
			}
		}
	}
	walk(root, 0, nil)
	return paths
}

func get(root *node, path []string) *node {
	for _, key := range path {
		if reserved(key) {
			return &node{scalar: Undefined{}}
		}
		root = root.child(key)
	}
	return root
}

func set(root *node, path []string, value *node) error {
	if len(path) == 0 || reserved(path[len(path)-1]) {
		return nil
	}
	for _, key := range path[:len(path)-1] {
		root = root.child(key)
		if root == nil {
			return &Error{"TypeError", "Masking path no longer exists", nil}
		}
	}
	key := path[len(path)-1]
	if root.object != nil {
		root.object[key] = value
		return nil
	}
	if root.array != nil {
		if key == "length" {
			return &Error{"RangeError", "Invalid array length", nil}
		}
		index, err := strconv.Atoi(key)
		if err == nil && index >= 0 && index < len(root.array) {
			root.array[index] = value
			return nil
		}
	}
	return &Error{"TypeError", "Masking path no longer exists", nil}
}

func pathKey(path []string) string {
	data, _ := json.Marshal(path)
	return string(data)
}
