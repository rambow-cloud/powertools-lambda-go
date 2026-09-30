package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type referenceStep struct {
	Op, Mode, Tag, Description string
	Name                       *string
	Value, Event               any
	Response                   map[string]any
}

func decodeSpecial(value any) any {
	switch v := value.(type) {
	case map[string]any:
		if tag, ok := v["$"].(string); ok {
			if tag == "undefined" {
				return Undefined{}
			}
			return commons.ParseNumber(tag)
		}
		out := map[string]any{}
		for key, item := range v {
			out[key] = decodeSpecial(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = decodeSpecial(item)
		}
		return out
	default:
		return value
	}
}

func normalize(value any) any {
	switch v := value.(type) {
	case Undefined:
		return map[string]any{"$": "undefined"}
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || (v == 0 && math.Signbit(v)) {
			tag := numberString(v, false)
			if v == 0 {
				tag = "-0"
			}
			return map[string]any{"$": tag}
		}
	case *Parameters:
		return normalize(v.Values())
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = normalize(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	}
	return value
}

func jsonValue(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func explicitResponse(input map[string]any) *FunctionResponse {
	body := any(Undefined{})
	if value, exists := input["body"]; exists {
		body = decodeSpecial(value)
	}
	r := NewFunctionResponse(body)
	fields := map[string]*any{"responseState": &r.ResponseState, "sessionAttributes": &r.SessionAttributes, "promptSessionAttributes": &r.PromptSessionAttributes, "knowledgeBasesConfiguration": &r.KnowledgeBasesConfiguration}
	for key, field := range fields {
		if value, exists := input[key]; exists {
			decoded := decodeSpecial(value)
			if _, absent := decoded.(Undefined); !absent {
				*field = decoded
			}
		}
	}
	return r
}

func TestTypeScriptReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Steps                []referenceStep
			Results, Calls, Logs []any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) < 350 {
		t.Fatalf("incomplete fixture: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			calls, logs, results := []any{}, []any{}, []any{}
			app := New(Options{Diagnostic: func(_ context.Context, level, message string, err error) {
				entry := []any{level, message}
				if err != nil {
					entry = append(entry, errorText(err))
				}
				logs = append(logs, entry)
			}})
			for _, step := range item.Steps {
				switch step.Op {
				case "register":
					name := "tool"
					if step.Name != nil {
						name = *step.Name
					}
					app.Tool(func(_ context.Context, params *Parameters, event Event) (any, error) {
						calls = append(calls, map[string]any{"params": normalize(params), "keys": params.Keys(), "tag": step.Tag, "request": "request", "session": event["sessionId"]})
						switch step.Mode {
						case "error":
							return nil, &NamedError{Name: "TypeError", Message: "tool failed"}
						case "throw":
							panic(decodeSpecial(step.Value))
						case "return":
							return decodeSpecial(step.Value), nil
						case "explicit":
							return explicitResponse(step.Response), nil
						case "tag":
							return step.Tag, nil
						case "params":
							params.Set("extra", "added")
							params.Delete("remove")
						case "mutate":
							event["sessionAttributes"].(map[string]any)["changed"] = true
							event["sessionAttributes"] = map[string]any{"replaced": true}
							event["promptSessionAttributes"] = map[string]any{"replaced": true}
							return "mutated", nil
						}
						return params, nil
					}, Configuration{Name: name, Description: step.Description})
				case "build":
					results = append(results, map[string]any{"value": explicitResponse(step.Response).Build("group", "function"), "error": nil})
				default:
					value, err := app.Resolve(context.Background(), commons.CloneValue(step.Event))
					entry := map[string]any{"value": value, "error": nil}
					if err != nil {
						entry["error"] = errorText(err)
					}
					results = append(results, entry)
				}
			}
			for name, pair := range map[string][2]any{"results": {results, item.Results}, "calls": {calls, item.Calls}, "logs": {logs, item.Logs}} {
				if actual := jsonValue(t, pair[0]); !reflect.DeepEqual(actual, pair[1]) {
					t.Fatalf("%s mismatch\nactual: %v\nexpected: %v", name, actual, pair[1])
				}
			}
		})
	}
}
