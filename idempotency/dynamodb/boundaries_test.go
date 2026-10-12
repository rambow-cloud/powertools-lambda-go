package dynamodb

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
)

func TestPersistenceBoundaryReference(t *testing.T) {
	data, err := os.ReadFile("../testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Now  int64 `json:"now"`
		Rows []struct {
			Name   string             `json:"name"`
			Custom bool               `json:"custom"`
			Record idempotency.Record `json:"record"`
			Item   map[string]any     `json:"item"`
			Read   idempotency.Record `json:"dynamodbRead"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Rows) != 16 {
		t.Fatal("unexpected persistence corpus")
	}
	bridge := []map[string]any{}
	for _, row := range fixtures.Rows {
		t.Run(row.Name+map[bool]string{true: "-custom", false: "-default"}[row.Custom], func(t *testing.T) {
			var captured map[string]any
			sdkClient := client(t, func(r *http.Request) (*http.Response, error) {
				if err := json.UnmarshalRead(r.Body, &captured); err != nil {
					t.Fatal(err)
				}
				if r.Header.Get("X-Amz-Target") == "DynamoDB_20120810.GetItem" {
					if captured["ConsistentRead"] != true {
						t.Fatal("weak read")
					}
					encoded, _ := json.Marshal(map[string]any{"Item": row.Item})
					return response(200, string(encoded)), nil
				}
				return response(200, `{}`), nil
			})
			o := Options{TableName: "records"}
			if row.Custom {
				o.StatusAttribute = "s"
				o.ExpiryAttribute = "e"
				o.InProgressExpiryAttribute = "ip"
				o.DataAttribute = "d"
				o.ValidationAttribute = "v"
			}
			store, err := New(sdkClient, o)
			if err != nil {
				t.Fatal(err)
			}
			got, err := store.Get(context.Background(), row.Record.Key)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Clone(), row.Read.Clone()) {
				// Object member order is outside the native DynamoDB value contract.
				actual, _ := json.Marshal(got)
				want, _ := json.Marshal(row.Read)
				var a, b any
				_ = json.Unmarshal(actual, &a)
				_ = json.Unmarshal(want, &b)
				if !reflect.DeepEqual(a, b) {
					t.Fatalf("got=%s want=%s", actual, want)
				}
			}
			if row.Record.Status == idempotency.Completed {
				err = store.Update(context.Background(), row.Record)
			} else {
				err = store.Put(context.Background(), row.Record, time.UnixMilli(fixtures.Now))
			}
			if err != nil {
				t.Fatal(err)
			}
			var item map[string]any
			if row.Record.Status == idempotency.InProgress {
				item = captured["Item"].(map[string]any)
			} else {
				item = captured["Key"].(map[string]any)
				values := captured["ExpressionAttributeValues"].(map[string]any)
				for alias, attribute := range captured["ExpressionAttributeNames"].(map[string]any) {
					item[attribute.(string)] = values[":"+alias[1:]]
				}
			}
			if !reflect.DeepEqual(item, row.Item) {
				a, _ := json.Marshal(item)
				b, _ := json.Marshal(row.Item)
				t.Fatalf("wire=%s want=%s", a, b)
			}
			expected := map[string]any{"id": row.Record.Key, "status": row.Record.Status, "expiration": row.Record.Expiration, "validation": row.Record.Validation}
			if row.Record.InProgressExpiration != 0 {
				expected["in_progress_expiration"] = row.Record.InProgressExpiration
			}
			if len(row.Record.Data) > 0 {
				expected["data"] = jsontext.Value(row.Record.Data)
			}
			bridge = append(bridge, map[string]any{"name": row.Name, "custom": row.Custom, "item": item, "expected": expected})
		})
	}
	if directory := os.Getenv("POWERTOOLS_IDEMPOTENCY_BRIDGE_DIR"); directory != "" {
		encoded, err := json.Marshal(bridge)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "dynamodb-go.json"), encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
