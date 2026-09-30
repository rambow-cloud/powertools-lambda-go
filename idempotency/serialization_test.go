package idempotency

import (
	"encoding/json"
	"testing"
)

type changingMarshaler struct{ calls int }

func (v *changingMarshaler) MarshalJSON() ([]byte, error) {
	v.calls++
	return json.Marshal(map[string]int{"value": v.calls})
}

func TestKeyUsesOneMarshalerSnapshot(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	manager, err := New(newMemory(), Options{KeyPrefix: "snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	payload := &changingMarshaler{}
	key, _, _, err := manager.Key(payload)
	if err != nil {
		t.Fatal(err)
	}
	want, _, _, err := manager.Key(json.RawMessage(`{"value":1}`))
	if err != nil || key != want || payload.calls != 1 {
		t.Fatalf("key=%s want=%s calls=%d error=%v", key, want, payload.calls, err)
	}
}

func TestCustomSerializerReceivesOriginalSelection(t *testing.T) {
	t.Setenv("POWERTOOLS_IDEMPOTENCY_DISABLED", "false")
	payload := &changingMarshaler{}
	calls := 0
	manager, err := New(newMemory(), Options{SerializeKey: func(value any) ([]byte, error) {
		calls++
		if value != payload {
			t.Fatal("custom serializer received a replacement value")
		}
		return []byte("application-key"), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := manager.Key(payload); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || payload.calls != 1 {
		t.Fatalf("serializer=%d marshaler=%d", calls, payload.calls)
	}
}
