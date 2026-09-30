package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestTypeScriptReference(t *testing.T) {
	cleanEnv(t)
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Records []Fields `json:"records"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	l := New(WithOutput(&b), WithServiceName("orders"), WithPersistentKeys(Fields{"source": "base"}))
	_ = l.Info("plain")
	l.AppendKeys(Fields{"order": 7})
	_ = l.Info("attributes", Fields{"source": "call"})
	l.RemoveKeys("order")
	_ = l.Debug("filtered")
	_ = l.Warn("warning")
	got := records(t, &b)
	for _, r := range got {
		delete(r, "timestamp")
	}
	if !reflect.DeepEqual(got, fixture.Records) {
		t.Fatalf("Go: %#v\nTypeScript: %#v", got, fixture.Records)
	}
}
