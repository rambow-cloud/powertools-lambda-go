package http

import (
	"encoding/json"
	nethttp "net/http"
	"strings"
)

// proxyResult recognizes the reference's extended-result shape after a single
// caller serialization. Additional data keys only qualify when body is present.
func proxyResult(raw []byte) (Response, bool, error) {
	doc := object(raw)
	var status float64
	if len(doc["statusCode"]) == 0 || null(doc["statusCode"]) || json.Unmarshal(doc["statusCode"], &status) != nil {
		return Response{}, false, nil
	}
	for _, name := range []string{"headers", "multiValueHeaders"} {
		if value, present := doc[name]; present && object(value) == nil {
			return Response{}, false, nil
		}
	}
	if value, present := doc["isBase64Encoded"]; present && !isBool(value) {
		return Response{}, false, nil
	}
	if _, hasBody := doc["body"]; !hasBody {
		for key := range doc {
			if !strings.Contains(" statusCode body headers multiValueHeaders isBase64Encoded cookies statusDescription ", " "+key+" ") {
				return Response{}, false, nil
			}
		}
	}
	result := Response{StatusCode: int(status), Headers: make(nethttp.Header), MultiValueHeaders: make(nethttp.Header)}
	if raw, present := doc["isBase64Encoded"]; present {
		encoded := string(raw) == "true"
		result.IsBase64Encoded = &encoded
	}
	if body, present := doc["body"]; present && !null(body) {
		if text, ok := textValue(body); ok {
			result.Body = text
		} else {
			result.Body = body
		}
	}
	err := visitObject(doc["headers"], func(name string, raw json.RawMessage) error {
		if null(raw) {
			return nil
		}
		value, err := headerValue(raw)
		if err != nil {
			return err
		}
		return setHeader(result.Headers, name, value, false)
	})
	if err != nil {
		return Response{}, true, err
	}
	err = visitObject(doc["multiValueHeaders"], func(name string, raw json.RawMessage) error {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		for _, raw := range values {
			value, err := headerValue(raw)
			if err != nil {
				return err
			}
			if err := validateHeader(name, value); err != nil {
				return err
			}
			result.MultiValueHeaders.Add(name, value)
		}
		return nil
	})
	if err == nil && len(doc["cookies"]) > 0 {
		err = json.Unmarshal(doc["cookies"], &result.Cookies)
	}
	return result, true, err
}
