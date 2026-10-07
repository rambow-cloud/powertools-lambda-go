package http

import (
	"errors"
	"io"
	"testing"
	"time"

	json "encoding/json/v2"
)

func TestResponseBodyUsesJSONV2Defaults(t *testing.T) {
	payload := struct {
		Flag  bool           `json:"flag,omitempty"`
		Count int            `json:"count,omitempty"`
		Items []string       `json:"items"`
		Meta  map[string]int `json:"meta"`
		Text  string         `json:"text"`
	}{Text: "<>&\u2028\u2029"}
	response, encoding, err := handlerResponse(Response{Body: payload, StatusCode: 200}, nil, 200, false)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || encoding.binary || string(body) != "{\"flag\":false,\"count\":0,\"items\":[],\"meta\":{},\"text\":\"<>&\u2028\u2029\"}" {
		t.Fatalf("response defaults: body=%s binary=%t error=%v", body, encoding.binary, err)
	}
}

func TestResponseBodyRejectsDurationWithoutFormat(t *testing.T) {
	_, _, err := handlerResponse(Response{Body: struct{ Delay time.Duration }{time.Second}}, nil, 200, false)
	var semantic *json.SemanticError
	if !errors.As(err, &semantic) {
		t.Fatalf("unformatted duration accepted: %v", err)
	}
}
