package validation

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// Object keys follow JavaScript enumeration: array indices first, then source
// order. Go maps use their stable JSON encoding order. RawMessage and structs
// retain their encoded field order.
type objectOrder map[string][]string

func readObjectOrder(value any) objectOrder {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	order := objectOrder{}
	var read func(string)
	read = func(path string) {
		token, _ := decoder.Token()
		switch token {
		case json.Delim('{'):
			var keys []string
			seen := map[string]bool{}
			for decoder.More() {
				token, _ := decoder.Token()
				key := token.(string)
				if !seen[key] {
					keys = append(keys, key)
					seen[key] = true
				}
				read(path + pointer([]string{key}))
			}
			_, _ = decoder.Token()
			commons.SortObjectKeys(keys)
			order[path] = keys
		case json.Delim('['):
			for i := 0; decoder.More(); i++ {
				read(path + "/" + strconv.Itoa(i))
			}
			_, _ = decoder.Token()
		}
	}
	read("")
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
