package avro

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	json "encoding/json/v2"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/kafka"
)

func TestSchemaBoundaryReference(t *testing.T) {
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Schema string `json:"schema"`
			Data   string `json:"data"`
			Error  struct {
				Name    string `json:"name"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 10 {
		t.Fatalf("incomplete invalid-schema corpus: %d", len(fixture.Cases))
	}
	for _, test := range fixture.Cases {
		t.Run(test.Schema, func(t *testing.T) {
			value, err := Deserialize(context.Background(), test.Data, test.Schema, nil)
			var failure *kafka.DeserializationError
			if value != nil || !errors.As(err, &failure) || failure.Unwrap() == nil || test.Error.Message == "" || failure.ErrorName() != test.Error.Name {
				t.Fatalf("invalid schema lost its category/cause: %v, %v", value, err)
			}
		})
	}
}

func TestZeroWidthCollectionsHaveFiniteAllocationBudget(t *testing.T) {
	const count = 1<<20 + 1
	data := append(binary.AppendVarint(nil, count), 0)
	value, err := Deserialize(context.Background(), base64.StdEncoding.EncodeToString(data), `{"type":"array","items":"null"}`, nil)
	var failure *kafka.DeserializationError
	if value != nil || !errors.As(err, &failure) || !strings.Contains(err.Error(), "1048576 items") {
		t.Fatalf("tiny zero-width array exceeded the finite item budget: length=%d, error=%v", boundaryLength(value), err)
	}
}

func boundaryLength(value any) int {
	if items, ok := value.([]any); ok {
		return len(items)
	}
	return 0
}

func TestZeroWidthAndNegativeBlockCollectionsRemainSupported(t *testing.T) {
	for _, negative := range []bool{false, true} {
		const count = 32
		header := int64(count)
		if negative {
			header = -header
		}
		data := binary.AppendVarint(nil, header)
		if negative {
			data = binary.AppendVarint(data, 0)
		}
		data = append(data, 0)
		value, err := Deserialize(context.Background(), base64.StdEncoding.EncodeToString(data), `{"type":"array","items":"null"}`, nil)
		if err != nil || boundaryLength(value) != count {
			t.Fatalf("valid zero-width array rejected: negative=%v, value=%v, error=%v", negative, value, err)
		}
	}
}

func TestCollectionBudgetIsSharedAcrossBlocks(t *testing.T) {
	for _, extra := range []bool{false, true} {
		data := binary.AppendVarint(nil, 1<<19)
		data = binary.AppendVarint(data, 1<<19)
		if extra {
			data = binary.AppendVarint(data, 1)
		}
		data = append(data, 0)
		value, err := Deserialize(context.Background(), base64.StdEncoding.EncodeToString(data), `{"type":"array","items":"null"}`, nil)
		if extra {
			if value != nil || err == nil || !strings.Contains(err.Error(), "1048576 items") {
				t.Fatalf("split blocks bypassed budget: %v", err)
			}
		} else if err != nil || boundaryLength(value) != 1<<20 {
			t.Fatalf("exact item budget rejected: length=%d, error=%v", boundaryLength(value), err)
		}
	}
}
