package commons_test

import (
	json "encoding/json/v2"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/internal/testfixture"
)

func TestCommonsBoundaryReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string           `json:"version"`
		Cases   []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != 71 {
		t.Fatalf("fixture %s/%d", fixture.Version, len(fixture.Cases))
	}
	for _, item := range fixture.Cases {
		t.Run(item["mode"].(string), func(t *testing.T) {
			var value any
			var err error
			switch item["mode"] {
			case "number":
				value, err = commons.DynamoDBNumber(item["input"].(string))
			case "attribute":
				if attribute := item["input"].(map[string]any); attribute["M"] != nil {
					if _, malformed := attribute["M"].([]any); malformed {
						// Go deliberately requires M to be an object. Upstream
						// Object.entries accepts the empty array; keep its fixture.
						_, err := commons.UnmarshallDynamoDB(map[string]any{"value": attribute})
						if err == nil || item["error"] != nil {
							t.Fatalf("strict M difference: %v/%v", item, err)
						}
						return
					}
				}
				var result map[string]any
				result, err = commons.UnmarshallDynamoDB(map[string]any{"value": item["input"]})
				value = result["value"]
			case "base64":
				value, err = commons.FromBase64(item["input"].(string), item["encoding"].(string))
			case "type":
				n := commons.ParseNumber(item["input"].(string))
				value = map[string]any{"truthy": commons.IsTruthy(n), "integer": commons.IsIntegerNumber(n), "type": commons.GetType(n)}
			default:
				t.Fatal("unknown mode")
			}
			testfixture.AssertOutcome(t, item, value, err)
		})
	}
}

func TestCommonsNativeOwnershipAndNumbers(t *testing.T) {
	for _, value := range []any{int(0), int64(0), uint32(0), float32(0), float32(math.NaN())} {
		got, err := commons.UnmarshallDynamoDB(map[string]any{"flag": map[string]any{"BOOL": value}})
		if err != nil || got["flag"] != false {
			t.Fatalf("native numeric BOOL %T: %v/%v", value, got, err)
		}
	}
	bytes := []byte{0, 255}
	input := map[string]any{"bytes": bytes, "words": []string{"a"}, "numbers": []float64{1}}
	copy := commons.CloneValue(input).(map[string]any)
	copy["bytes"].([]byte)[0] = 9
	copy["words"].([]string)[0] = "changed"
	copy["numbers"].([]float64)[0] = 2
	if !reflect.DeepEqual(input, map[string]any{"bytes": []byte{0, 255}, "words": []string{"a"}, "numbers": []float64{1}}) {
		t.Fatal("native snapshot aliases caller")
	}
	lru := commons.NewLRUCache[string, map[string]int](2)
	shared := map[string]int{"value": 1}
	lru.Add("a", shared)
	lru.Add("b", map[string]int{})
	if !lru.Has("a") {
		t.Fatal("missing a")
	}
	lru.Add("c", map[string]int{})
	if lru.Has("a") {
		t.Fatal("Has refreshed eviction order")
	}
	lru.Add("b", shared)
	got, _ := lru.Get("b")
	got["value"] = 2
	if shared["value"] != 2 {
		t.Fatal("LRU unexpectedly cloned caller-owned value")
	}
	lru.Remove("missing")
	lru.Clear()
	lru.Add("new", shared)
	if lru.Size() != 1 || lru.Has("b") {
		t.Fatal("clear/update ownership")
	}
}
