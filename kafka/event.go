package kafka

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"sort"
	"strconv"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func eventObject(input any) (map[string]any, []string, error) {
	var raw []byte
	event, native := input.(map[string]any)
	if !native {
		var err error
		raw, err = json.Marshal(input, json.Deterministic(true))
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
		var fields map[string]jsonv1.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, nil, err
		}
		decoder := jsontext.NewDecoder(bytes.NewReader(fields["records"]))
		if _, err := decoder.ReadToken(); err != nil {
			return nil, nil, err
		}
		for decoder.PeekKind() != '}' {
			token, err := decoder.ReadToken()
			if err != nil {
				return nil, nil, err
			}
			key := token.String()
			if err := decoder.SkipValue(); err != nil {
				return nil, nil, err
			}
			keys = append(keys, key)
		}
		if _, err := decoder.ReadToken(); err != nil {
			return nil, nil, err
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
