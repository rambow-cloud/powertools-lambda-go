package metrics

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type boundaryMarshaler struct {
	calls   int
	marshal func() ([]byte, error)
}

func TestMetadataBoundaryReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string `json:"version"`
		Now     int64  `json:"now"`
		Cases   []struct {
			Name     string         `json:"name"`
			Key      string         `json:"key"`
			Input    any            `json:"input"`
			Special  string         `json:"special"`
			Expected map[string]any `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != 15 {
		t.Fatal("unexpected metadata fixture")
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			m, _ := setup(t, WithDefaultDimensions(Dimensions{}), WithClock(func() time.Time { return time.UnixMilli(fixture.Now) }))
			if item.Special != "" {
				value := math.NaN()
				if item.Special == "Infinity" {
					value = math.Inf(1)
				}
				if item.Special == "-Infinity" {
					value = math.Inf(-1)
				}
				// Strict Go JSON v2 rejects native non-finite metadata. The
				// reference emits null; preserve its complete observation.
				upstream, present := item.Expected["payload"]
				if !present || upstream != nil {
					t.Fatal("pinned non-finite observation changed")
				}
				if err := m.AddMetadata(item.Key, value); err == nil {
					t.Fatal("non-finite native metadata accepted")
				}
				return
			}
			snapshot := commons.CloneValue(item.Input)
			if err := m.AddMetadata(item.Key, item.Input); err != nil {
				t.Fatal(err)
			}
			if item.Name == "mutation-object" {
				item.Input.(map[string]any)["nested"].(map[string]any)["value"] = "new"
			}
			if item.Name == "mutation-array" {
				item.Input.([]any)[0] = "new"
			}
			if err := m.AddMetric("Count", Count, 1); err != nil {
				t.Fatal(err)
			}
			data, err := m.Serialize()
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatal(err)
			}
			if item.Name == "mutation-object" || item.Name == "mutation-array" {
				if reflect.DeepEqual(item.Expected["payload"], snapshot) {
					t.Fatal("pinned live-reference mutation changed")
				}
				// Go's documented insertion snapshot intentionally retains old.
				item.Expected["payload"] = snapshot
			}
			if !reflect.DeepEqual(actual, item.Expected) {
				t.Fatalf("actual=%s; upstream/native decision=%v", data, item.Expected)
			}
		})
	}
}

func (m *boundaryMarshaler) MarshalJSON() ([]byte, error) { m.calls++; return m.marshal() }

func TestClosedMetadataRejectsBeforeSerialization(t *testing.T) {
	m, _ := setup(t)
	ctx, finish := m.StartScope(context.Background())
	bound := m.WithContext(ctx)
	if err := bound.AddMetric("Count", Count, 1); err != nil {
		t.Fatal(err)
	}
	if err := finish(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"payload", "__proto__"} {
		value := &boundaryMarshaler{marshal: func() ([]byte, error) { return nil, errors.New("should not serialize") }}
		if err := bound.AddMetadata(key, value); !errors.Is(err, ErrInvocationClosed) || value.calls != 0 {
			t.Fatalf("closed metadata key=%s: err=%v calls=%d", key, err, value.calls)
		}
	}
}

func TestMetadataSnapshotPrecisionAndAtomicFailures(t *testing.T) {
	m, _ := setup(t)
	value := map[string]any{"items": []any{"old"}}
	if err := m.AddMetadata("payload", value); err != nil {
		t.Fatal(err)
	}
	value["items"].([]any)[0] = "new"
	if err := m.AddMetadata("large", int64(9007199254740993)); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("native marshaler failed")
	marshal := &boundaryMarshaler{marshal: func() ([]byte, error) { return nil, failure }}
	if err := m.AddMetadata("payload", marshal); !errors.Is(err, failure) || marshal.calls != 1 {
		t.Fatalf("marshal cause/count: %v/%d", err, marshal.calls)
	}
	for _, bad := range []any{math.NaN(), math.Inf(1), jsontext.Value(`{"a":1,"a":2}`), jsontext.Value("\"\xff\""), make(chan int)} {
		if err := m.AddMetadata("payload", bad); err == nil {
			t.Fatalf("invalid native metadata accepted: %T", bad)
		}
	}
	if err := m.AddMetric("Count", Count, 1); err != nil {
		t.Fatal(err)
	}
	data, err := m.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]jsontext.Value
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if string(document["large"]) != "9007199254740993" || string(document["payload"]) != `{"items":["old"]}` {
		t.Fatalf("snapshot/precision changed: %s", data)
	}
	document["payload"][0] = '['
	next, err := m.Serialize()
	if err != nil || !strings.Contains(string(next), `"items":["old"]`) {
		t.Fatal("serialized output aliases store")
	}
}

func TestMetadataCloseDuringSerialization(t *testing.T) {
	m, out := setup(t)
	ctx, finish := m.StartScope(context.Background())
	bound := m.WithContext(ctx)
	if err := bound.AddMetric("Count", Count, 1); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	value := &boundaryMarshaler{marshal: func() ([]byte, error) { close(entered); <-release; return []byte(`"late"`), nil }}
	done := make(chan error, 1)
	go func() { done <- bound.AddMetadata("payload", value) }()
	<-entered
	if err := finish(); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	if err := <-done; !errors.Is(err, ErrInvocationClosed) || value.calls != 1 {
		t.Fatalf("close during serialization: %v/%d", err, value.calls)
	}
	if strings.Contains(out.String(), "payload") {
		t.Fatal("metadata escaped its finished scope")
	}
}

func TestMultiInstanceConcurrentMetadataIsolation(t *testing.T) {
	a, outA := setup(t)
	b, outB := setup(t)
	targets := []*Metrics{a, b}
	dimensions := Dimensions{"tenant": "original"}
	var retained sync.Map
	handler := WrapHandlers(targets, func(ctx context.Context, id int) (int, error) {
		for _, m := range []*Metrics{a, b} {
			bound := m.WithContext(ctx)
			retained.Store(struct {
				m  *Metrics
				id int
			}{m, id}, bound)
			if err := bound.AddMetadata("id", id); err != nil {
				return 0, err
			}
			if err := bound.AddMetric("Count", Count, 1); err != nil {
				return 0, err
			}
		}
		return id, nil
	}, HandlerOptions{DefaultDimensions: dimensions})
	targets[0] = nil
	dimensions["tenant"] = "changed"
	var group sync.WaitGroup
	for id := range 32 {
		group.Go(func() {
			result, err := handler(context.Background(), id)
			if err != nil || result != id {
				t.Errorf("outcome: %d/%v", result, err)
			}
		})
	}
	group.Wait()
	for _, out := range []*bytes.Buffer{outA, outB} {
		docs := documents(t, out)
		if len(docs) != 32 {
			t.Fatalf("documents=%d", len(docs))
		}
		seen := map[float64]bool{}
		for _, doc := range docs {
			id := doc["id"].(float64)
			if seen[id] || doc["tenant"] != "original" || doc["Count"] != float64(1) {
				t.Fatalf("scope leak: %v", doc)
			}
			seen[id] = true
		}
	}
	retained.Range(func(_, value any) bool {
		if err := value.(*Metrics).AddMetadata("late", true); !errors.Is(err, ErrInvocationClosed) {
			t.Errorf("late metadata: %v", err)
		}
		return true
	})
}
