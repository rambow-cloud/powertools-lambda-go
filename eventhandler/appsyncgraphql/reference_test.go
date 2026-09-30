package appsyncgraphql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

type step struct {
	Op, Target, Kind, Mode, Field, Tag string
	TypeName                           *string
	Individual, ThrowOnError           bool
	Names, Routers                     []string
	Event                              any
}
type reference struct {
	Cases []struct {
		Steps                []step
		Results, Calls, Logs []any
	}
	Scalars []struct {
		Millis    int64
		Offset    any
		Timestamp int64
		Values    map[string]any
	}
}

func readReference(t *testing.T) reference {
	t.Helper()
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var result reference
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func jsonValue(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func failure(err error) string {
	var unknown thrownValue
	if errors.As(err, &unknown) {
		return "An unknown error occurred"
	}
	return errorName(err) + " - " + err.Error()
}

func TestTypeScriptReference(t *testing.T) {
	fixture := readReference(t)
	if len(fixture.Cases) < 100 {
		t.Fatal("incomplete resolver fixture")
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			logs, calls, results := []any{}, []any{}, []any{}
			options := Options{Diagnostic: func(_ context.Context, level, message string, err error) {
				entry := []any{level, message}
				if level == "error" {
					entry = append(entry, failure(err))
				}
				logs = append(logs, entry)
			}}
			app := New(options)
			routers := map[string]*Router{"app": app.Router, "a": NewRouter(options), "b": NewRouter(options)}
			for _, current := range item.Steps {
				target := current.Target
				if target == "" {
					target = "app"
				}
				router := routers[target]
				switch current.Op {
				case "register":
					handler := func(_ context.Context, input, event any) (any, error) {
						field := "batch"
						if isEvent(event) {
							_, field = eventRoute(event)
						}
						calls = append(calls, map[string]any{"tag": current.Tag, "input": input, "field": field, "request": "request"})
						args, _ := input.(map[string]any)
						switch current.Mode {
						case "error":
							return nil, &NamedError{Name: "TypeError", Message: "handler failed"}
						case "mixed":
							if args["fail"] == true {
								return nil, &NamedError{Name: "TypeError", Message: "handler failed"}
							}
						case "missing":
							return nil, &ResolverNotFoundException{Message: "explicit missing"}
						case "invalid":
							return nil, &InvalidBatchResponseException{Message: "explicit invalid"}
						case "unknown":
							panic(42)
						case "null", "undefined":
							return nil, nil
						case "array":
							return []string{"short"}, nil
						case "tag":
							value := map[string]any{"input": input}
							if current.Tag != "" {
								value["tag"] = current.Tag
							}
							return value, nil
						}
						return input, nil
					}
					typeName := "Query"
					if current.TypeName != nil {
						typeName = *current.TypeName
					}
					if current.Kind == "batch" {
						router.OnBatchResolver(typeName, current.Field, handler, BatchOptions{Individual: current.Individual, ThrowOnError: current.ThrowOnError})
					} else {
						router.OnResolver(typeName, current.Field, handler)
					}
				case "exception":
					router.OnException(current.Names, func(_ context.Context, err error) (any, error) {
						calls = append(calls, map[string]any{"exception": errorName(err), "tag": current.Tag})
						if current.Mode == "error" {
							return nil, errors.New("exception failed")
						}
						if current.Mode == "unknown" {
							panic(42)
						}
						return map[string]any{"handled": err.Error(), "tag": current.Tag}, nil
					})
				case "include":
					included := []*Router{}
					for _, name := range current.Routers {
						included = append(included, routers[name])
					}
					app.IncludeRouter(included...)
				default:
					value, err := app.Resolve(context.Background(), current.Event)
					entry := map[string]any{"value": value, "error": nil}
					if err != nil {
						entry["error"] = failure(err)
					}
					results = append(results, entry)
				}
			}
			for name, pair := range map[string][2]any{"results": {results, item.Results}, "calls": {calls, item.Calls}, "logs": {logs, item.Logs}} {
				actual := jsonValue(t, pair[0])
				if !reflect.DeepEqual(actual, pair[1]) {
					t.Fatalf("%s mismatch\nactual: %v\nexpected: %v", name, actual, pair[1])
				}
			}
		})
	}
}

func TestScalarReference(t *testing.T) {
	fixture := readReference(t)
	if len(fixture.Scalars) != 91 {
		t.Fatal("incomplete scalar fixture")
	}
	for index, item := range fixture.Scalars {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			now := time.UnixMilli(item.Millis)
			offset, _ := item.Offset.(float64)
			if name, ok := item.Offset.(string); ok {
				switch name {
				case "NaN":
					offset = math.NaN()
				case "Infinity":
					offset = math.Inf(1)
				case "-Infinity":
					offset = math.Inf(-1)
				}
			}
			if actual := AWSTimestamp(now); actual != item.Timestamp {
				t.Fatalf("timestamp: %d != %d", actual, item.Timestamp)
			}
			for name, fn := range map[string]func(time.Time, ...float64) (string, error){"date": AWSDate, "time": AWSTime, "datetime": AWSDateTime} {
				value, err := fn(now, offset)
				actual := map[string]any{"value": value, "error": nil}
				if err != nil {
					actual["value"], actual["error"] = nil, failure(err)
				}
				if !reflect.DeepEqual(actual, item.Values[name]) {
					t.Fatalf("%s: %v != %v", name, actual, item.Values[name])
				}
			}
		})
	}
}
