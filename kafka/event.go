package kafka

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func eventObject(input any) (map[string]any, []string, error) {
	var raw []byte
	event, native := input.(map[string]any)
	if !native {
		var err error
		raw, err = json.Marshal(input)
		if err != nil {
			return nil, nil, err
		}
		decoded, err := parseJSON(raw)
		if err != nil {
			return nil, nil, err
		}
		event, _ = decoded.(map[string]any)
	}
	if event == nil {
		return nil, nil, invalidEvent()
	}
	topics, ok := event["records"].(map[string]any)
	if !ok {
		return nil, nil, invalidEvent()
	}
	keys := make([]string, 0, len(topics))
	if raw == nil {
		for key := range topics {
			keys = append(keys, key)
		}
		sort.Strings(keys)
	} else {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(fields["records"]))
		if _, err := decoder.Token(); err != nil {
			return nil, nil, err
		}
		seen := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return nil, nil, err
			}
			key := token.(string)
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return nil, nil, err
			}
			if !seen[key] {
				keys = append(keys, key)
				seen[key] = true
			}
		}
	}
	commons.SortObjectKeys(keys)
	return event, keys, nil
}

func properties(value any) map[string]any {
	if fields, ok := value.(map[string]any); ok {
		return fields
	}
	fields := map[string]any{}
	switch v := value.(type) {
	case []any:
		for i, item := range v {
			fields[strconv.Itoa(i)] = item
		}
	case string:
		for i, item := range []rune(v) {
			fields[strconv.Itoa(i)] = string(item)
		}
	}
	return fields
}
