package envelopes

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"sort"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

type kafkaEnvelope[T any] struct{ payload parser.Schema[T] }

// Kafka flattens topic-partition records after Base64-to-text decoding.
// RawMessage input preserves JSON topic order; Go maps use sorted topic keys.
func Kafka[T any](payload parser.Schema[T]) parser.Schema[[]T] { return kafkaEnvelope[T]{payload} }
func (s kafkaEnvelope[T]) Validate(ctx context.Context, input any) ([]T, []parser.Issue, error) {
	return s.validate(ctx, input, false)
}
func (s kafkaEnvelope[T]) ValidateSafe(ctx context.Context, input any) ([]T, []parser.Issue, error) {
	return s.validate(ctx, input, true)
}

var kafkaSource = parser.Object(parser.Field{Name: "eventSource", Schema: parser.Union(parser.Literal("aws:kafka"), parser.Literal("SelfManagedKafka"))})

func (s kafkaEnvelope[T]) validate(ctx context.Context, input any, safe bool) ([]T, []parser.Issue, error) {
	if s.payload == nil {
		return nil, nil, fmt.Errorf("Kafka payload schema is required")
	}
	raw, err := json.Marshal(input, json.Deterministic(true))
	if err != nil {
		return nil, nil, err
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return nil, nil, err
	}
	if safe && decoded == nil {
		return nil, nil, fmt.Errorf("cannot read Kafka eventSource from null")
	}
	if !safe {
		_, issues, err := kafkaSource.Validate(ctx, decoded)
		if err != nil {
			return nil, nil, err
		}
		if issues != nil {
			return nil, nil, &parser.ParseError{Message: "Failed to parse Kafka event source", Issues: issues}
		}
	}
	source := ""
	if object, ok := decoded.(map[string]any); ok {
		source, _ = object["eventSource"].(string)
	}
	var outer parser.Schema[any] = schemas.KafkaSelfManagedEventSchema
	if source == "aws:kafka" {
		outer = schemas.KafkaMskEventSchema
	}
	parsed, issues, err := outer.Validate(ctx, decoded)
	if err != nil {
		return nil, nil, err
	}
	keys := kafkaTopicKeys(raw)
	if issues != nil {
		orderKafkaIssues(issues, keys)
		return nil, nil, &parser.ParseError{Message: "Failed to parse Kafka envelope", Issues: issues}
	}
	records := parsed.(map[string]any)["records"].(map[string]any)
	result := make([]T, 0)
	var failures []parser.Issue
	for _, key := range keys {
		for _, record := range records[key].([]any) {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			var value T
			var issues []parser.Issue
			var err error
			if extended, ok := s.payload.(parser.SafeSchema[T]); safe && ok {
				value, issues, err = extended.ValidateSafe(ctx, record.(map[string]any)["value"])
			} else {
				value, issues, err = s.payload.Validate(ctx, record.(map[string]any)["value"])
			}
			if err != nil {
				return nil, nil, err
			}
			if issues != nil {
				if !safe {
					return nil, nil, &parser.ParseError{Message: "Failed to parse Kafka record", Issues: issues}
				}
				if failures == nil {
					failures = []parser.Issue{}
				}
				failures = append(failures, parser.Prefix(issues, "records", key)...)
			} else {
				result = append(result, value)
			}
		}
	}
	if failures != nil {
		return nil, nil, &parser.ParseError{Message: "Failed to parse Kafka envelope", Issues: failures}
	}
	return result, nil, nil
}

func kafkaTopicKeys(raw []byte) []string {
	var event map[string]jsonv1.RawMessage
	if json.Unmarshal(raw, &event) != nil {
		return nil
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(event["records"]))
	if token, err := decoder.ReadToken(); err != nil || token.Kind() != '{' {
		return nil
	}
	keys := []string{}
	for decoder.PeekKind() != '}' {
		token, err := decoder.ReadToken()
		if err != nil {
			return nil
		}
		key := token.String()
		if decoder.SkipValue() != nil {
			return nil
		}
		keys = append(keys, key)
	}
	if _, err := decoder.ReadToken(); err != nil {
		return nil
	}
	commons.SortObjectKeys(keys)
	return keys
}

func orderKafkaIssues(issues []parser.Issue, keys []string) {
	rank := make(map[string]int, len(keys))
	for i, key := range keys {
		rank[key] = i
	}
	sort.SliceStable(issues, func(i, j int) bool {
		left, right := issues[i].Path, issues[j].Path
		if len(left) < 2 || len(right) < 2 || left[0] != "records" || right[0] != "records" {
			return false
		}
		a, aok := left[1].(string)
		b, bok := right[1].(string)
		return aok && bok && rank[a] < rank[b]
	})
}
