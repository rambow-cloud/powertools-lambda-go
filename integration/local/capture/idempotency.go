package main

import (
	json "encoding/json/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// This fixture implements only requests emitted by the idempotency adapter.
// It verifies runtime composition; it does not prove DynamoDB service behavior.
type idempotencyFixture struct {
	mu       sync.Mutex
	records  map[string]map[string]any
	requests []map[string]string
}

func newIdempotencyFixture() *idempotencyFixture {
	return &idempotencyFixture{records: map[string]map[string]any{}, requests: []map[string]string{}}
}
func (f *idempotencyFixture) snapshot() any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return map[string]any{"record_count": len(f.records), "requests": append([]map[string]string(nil), f.requests...)}
}
func avString(item map[string]any, key, kind string) string {
	value, _ := item[key].(map[string]any)
	result, _ := value[kind].(string)
	return result
}
func avNumber(item map[string]any, key string) float64 {
	value, _ := strconv.ParseFloat(avString(item, key, "N"), 64)
	return value
}

func (f *idempotencyFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	target := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "DynamoDB_20120810.")
	f.requests = append(f.requests, map[string]string{"operation": target, "traceparent": r.Header.Get("Traceparent"), "user_agent": r.Header.Get("User-Agent")})
	var input struct {
		TableName                 string
		Item, Key                 map[string]any
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]any
	}
	if err := json.UnmarshalRead(r.Body, &input); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	if input.TableName != "local-idempotency" {
		http.Error(w, "unexpected table", 400)
		return
	}
	key := avString(input.Key, "id", "S")
	switch target {
	case "PutItem":
		key = avString(input.Item, "id", "S")
		existing, found := f.records[key]
		now := avNumber(input.ExpressionAttributeValues, ":now")
		millis := avNumber(input.ExpressionAttributeValues, ":now_in_millis")
		expired := avNumber(existing, "expiration") < now
		lease := avNumber(existing, "in_progress_expiration")
		orphan := avString(existing, "status", "S") == "INPROGRESS" && lease != 0 && lease < millis
		if found && !expired && !orphan {
			w.WriteHeader(400)
			_ = json.MarshalWrite(w, map[string]any{"__type": "ConditionalCheckFailedException", "Item": existing})
			return
		}
		f.records[key] = input.Item
	case "GetItem":
		_ = json.MarshalWrite(w, map[string]any{"Item": f.records[key]})
		return
	case "UpdateItem":
		item := f.records[key]
		if item == nil {
			item = input.Key
		}
		for alias, name := range input.ExpressionAttributeNames {
			item[name] = input.ExpressionAttributeValues[":"+strings.TrimPrefix(alias, "#")]
		}
		f.records[key] = item
	case "DeleteItem":
		delete(f.records, key)
	default:
		http.Error(w, "unsupported fixture operation", 400)
		return
	}
	_ = json.MarshalWrite(w, map[string]any{})
}
