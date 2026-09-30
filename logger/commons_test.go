package logger

import (
	"io"
	"reflect"
	"testing"
)

func TestSharedMergePreservesArraysAndChildIsolation(t *testing.T) {
	l := New(WithOutput(io.Discard))
	source := Fields{"list": []any{map[string]any{"first": 1}, 2, 3}, "nested": Fields{"first": 1}}
	l.AppendPersistentKeys(source)
	child := l.Child()
	child.AppendPersistentKeys(Fields{"list": []any{Fields{"second": 2}, 9}, "nested": Fields{"second": 2}, "service": "ignored"})
	got := child.PersistentKeys()
	if !reflect.DeepEqual(got["list"], []any{map[string]any{"first": 1, "second": 2}, 9, 3}) {
		t.Fatalf("indexed merge: %v", got)
	}
	if !reflect.DeepEqual(l.PersistentKeys()["list"], source["list"]) {
		t.Fatal("child merge changed parent")
	}
	if _, ok := got["service"]; ok {
		t.Fatal("reserved field not filtered")
	}
	l.AppendKeys(Fields{"nested": Fields{"a": 1}, "list": []any{1, 2}})
	l.AppendKeys(Fields{"nested": Fields{"b": 2}, "list": []any{3}})
	if !reflect.DeepEqual(l.current.temporary["list"], []any{3, 2}) || !reflect.DeepEqual(l.current.temporary["nested"], map[string]any{"a": 1, "b": 2}) {
		t.Fatalf("temporary merge: %v", l.current.temporary)
	}
}

func TestSharedConfigurationModes(t *testing.T) {
	t.Setenv("POWERTOOLS_DEV", " yes ")
	t.Setenv("POWERTOOLS_LOGGER_LOG_EVENT", "yes")
	t.Setenv("POWERTOOLS_LOGGER_SAMPLE_RATE", " 0.5 ")
	t.Setenv("POWERTOOLS_SERVICE_NAME", " orders ")
	l := New(WithOutput(io.Discard), WithRandom(func() int { return 99 }))
	if !l.cfg.pretty || l.cfg.logEvent || l.cfg.rate != 0.5 || l.cfg.service != "orders" {
		t.Fatalf("configuration modes: %#v", l.cfg)
	}
}
