package logger

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReplacerThroughPointers(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	secret := Fields{"password": "private"}
	items := []any{&secret}
	calls := 0
	l := New(WithOutput(&out), WithReplacer(func(key string, value any) any {
		if key == "payload" {
			calls++
		}
		if key == "password" {
			return "[REDACTED]"
		}
		return value
	}))
	if err := l.Info("redaction", Fields{"payload": &items}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "private") || !strings.Contains(out.String(), "[REDACTED]") || calls != 1 {
		t.Fatalf("pointer redaction failed (%d calls): %s", calls, out.String())
	}
	if secret["password"] != "private" {
		t.Fatal("replacer mutated caller data")
	}
	var cycle any
	cycle = &cycle
	out.Reset()
	if err := l.Info("cycle", Fields{"payload": cycle}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[Circular]") {
		t.Fatal(out.String())
	}
}

func TestFormatterFailureAndReplacement(t *testing.T) {
	cleanEnv(t)
	var out bytes.Buffer
	want := errors.New("formatter failed")
	l := New(WithOutput(&out), WithFormatter(func(Fields) (any, error) { return nil, want }))
	if err := l.Info("test"); !errors.Is(err, want) || out.Len() != 0 {
		t.Fatalf("error=%v output=%s", err, &out)
	}
	l = New(WithOutput(&out), WithFormatter(func(f Fields) (any, error) {
		return Fields{"text": f["message"], "password": "private"}, nil
	}), WithReplacer(func(key string, value any) any {
		if key == "password" {
			return "hidden"
		}
		return value
	}))
	if err := l.Info("test"); err != nil {
		t.Fatal(err)
	}
	r := records(t, &out)
	if r[0]["text"] != "test" || r[0]["password"] != "hidden" || len(r[0]) != 2 {
		t.Fatal(r)
	}
}

func TestTimezoneConfiguration(t *testing.T) {
	for _, tc := range []struct{ zone, want string }{
		{"", "2026-01-01T00:00:00.000Z"},
		{"Asia/Hong_Kong", "2026-01-01T08:00:00.000+08:00"},
		{"America/New_York", "2025-12-31T19:00:00.000-05:00"},
		{"Etc/UTC", "2026-01-01T00:00:00.000Z"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("TZ", tc.zone)
			var out bytes.Buffer
			l := New(WithOutput(&out), WithClock(func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }))
			if err := l.Info("time"); err != nil {
				t.Fatal(err)
			}
			if got := records(t, &out)[0]["timestamp"]; got != tc.want {
				t.Fatalf("got %v, want %s", got, tc.want)
			}
		})
	}
}

func TestLevelStreamRouting(t *testing.T) {
	cleanEnv(t)
	var stdout, stderr bytes.Buffer
	l := New(WithLevel(TraceLevel), func(c *config) { c.stdout, c.stderr = &stdout, &stderr })
	for _, level := range []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel} {
		if err := l.Log(level, level.String()); err != nil {
			t.Fatal(err)
		}
	}
	low, high := records(t, &stdout), records(t, &stderr)
	if len(low) != 3 || len(high) != 3 || low[2]["level"] != "INFO" || high[0]["level"] != "WARN" {
		t.Fatalf("stdout=%s stderr=%s", &stdout, &stderr)
	}
}
