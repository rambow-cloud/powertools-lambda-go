package validation

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strconv"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// Object keys follow JavaScript enumeration: array indices first, then source
// order. Go maps use their stable JSON encoding order. RawMessage and structs
// retain their encoded field order.
type objectOrder map[string][]string

func readObjectOrder(value any) objectOrder {
	raw, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return nil
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(raw))
	order := objectOrder{}
	var read func(string) error
	read = func(path string) error {
		token, err := decoder.ReadToken()
		if err != nil {
			return err
		}
		switch token.Kind() {
		case '{':
			keys := []string{}
			for decoder.PeekKind() != '}' {
				token, err := decoder.ReadToken()
				if err != nil {
					return err
				}
				key := token.String()
				keys = append(keys, key)
				if err := read(path + pointer([]string{key})); err != nil {
					return err
				}
			}
			if _, err := decoder.ReadToken(); err != nil {
				return err
			}
			commons.SortObjectKeys(keys)
			order[path] = keys
		case '[':
			for i := 0; decoder.PeekKind() != ']'; i++ {
				if err := read(path + "/" + strconv.Itoa(i)); err != nil {
					return err
				}
			}
			if _, err := decoder.ReadToken(); err != nil {
				return err
			}
		}
		return nil
	}
	if read("") != nil {
		return nil
	}
	return order
}

func (o objectOrder) index(path, key string) int {
	for i, name := range o[path] {
		if name == key {
			return i
		}
	}
	return len(o[path])
}
