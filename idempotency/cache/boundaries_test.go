package cache

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
)

func TestLeaseEqualityCannotBeRecovered(t *testing.T) {
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Now        int64 `json:"now"`
		EqualLease struct {
			Seed           string `json:"seed"`
			ReferenceFinal string `json:"referenceFinal"`
		} `json:"equalLease"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.EqualLease.Seed == fixtures.EqualLease.ReferenceFinal || fixtures.EqualLease.Seed == "" {
		t.Fatal("upstream equality fixture did not perform takeover")
	}
	for _, offset := range []int64{-1, 0, 1} {
		t.Run(time.Duration(offset).String(), func(t *testing.T) {
			fake := newFake()
			fake.values["audit#key"] = fixtures.EqualLease.Seed
			store, _ := New(fake, Options{})
			err := store.Put(context.Background(), idempotency.Record{Key: "audit#key", Status: idempotency.InProgress, Expiration: 1800003600, InProgressExpiration: fixtures.Now + 5000}, time.UnixMilli(fixtures.Now+offset))
			if offset > 0 {
				if err != nil || fake.values["audit#key"] == fixtures.EqualLease.Seed {
					t.Fatalf("expired lease was not recovered: %v", err)
				}
				return
			}
			var conflict *idempotency.AlreadyExistsError
			if !errors.As(err, &conflict) || conflict.Record == nil || fake.values["audit#key"] != fixtures.EqualLease.Seed || len(fake.values) != 1 {
				t.Fatalf("live lease overwritten: error=%v records=%v", err, fake.values)
			}
		})
	}
}

func TestPersistenceBoundaryReference(t *testing.T) {
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Now  int64 `json:"now"`
		Rows []struct {
			Name      string             `json:"name"`
			Custom    bool               `json:"custom"`
			Record    idempotency.Record `json:"record"`
			Cache     string             `json:"cache"`
			CacheRead idempotency.Record `json:"cacheRead"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Rows) != 16 {
		t.Fatal("unexpected persistence corpus")
	}
	now := time.UnixMilli(fixtures.Now)
	bridge := []map[string]any{}
	for _, row := range fixtures.Rows {
		t.Run(row.Name+map[bool]string{true: "-custom", false: "-default"}[row.Custom], func(t *testing.T) {
			fake := newFake()
			o := Options{Now: func() time.Time { return now }}
			if row.Custom {
				o.StatusAttribute = "s"
				o.ExpiryAttribute = "e"
				o.InProgressExpiryAttribute = "ip"
				o.DataAttribute = "d"
				o.ValidationAttribute = "v"
			}
			store, err := New(fake, o)
			if err != nil {
				t.Fatal(err)
			}
			fake.values[row.Record.Key] = row.Cache
			got, err := store.Get(context.Background(), row.Record.Key)
			if err != nil {
				t.Fatal(err)
			}
			equivalent(t, got, row.CacheRead)
			delete(fake.values, row.Record.Key)
			if row.Record.Status == idempotency.Completed {
				err = store.Update(context.Background(), row.Record)
			} else {
				err = store.Put(context.Background(), row.Record, now)
			}
			if err != nil {
				t.Fatal(err)
			}
			// The Go default retains validation omitted by the pinned cache writer.
			var expected map[string]any
			if err := json.Unmarshal([]byte(row.Cache), &expected); err != nil {
				t.Fatal(err)
			}
			expected[store.options.ValidationAttribute] = row.Record.Validation
			var actual any
			if err := json.Unmarshal([]byte(fake.values[row.Record.Key]), &actual); err != nil {
				t.Fatal(err)
			}
			equivalent(t, actual, expected)
			read, err := store.Get(context.Background(), row.Record.Key)
			if err != nil {
				t.Fatal(err)
			}
			if read.Validation != row.Record.Validation {
				t.Fatal("Go record lost validation")
			}
			normalized := map[string]any{"id": read.Key, "status": read.Status, "expiration": read.Expiration, "validation": read.Validation}
			if read.InProgressExpiration != 0 {
				normalized["in_progress_expiration"] = read.InProgressExpiration
			}
			if len(read.Data) > 0 {
				normalized["data"] = jsontext.Value(read.Data)
			}
			bridge = append(bridge, map[string]any{"name": row.Name, "custom": row.Custom, "value": fake.values[row.Record.Key], "expected": normalized})
		})
	}
	if directory := os.Getenv("POWERTOOLS_IDEMPOTENCY_BRIDGE_DIR"); directory != "" {
		encoded, err := json.Marshal(bridge)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "cache-go.json"), encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
