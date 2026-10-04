package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

// The action corpus runs identical public operations against the actual pinned
// TypeScript package and Go; expected records are not derived from Go internals.
func TestLoggerParityReference(t *testing.T) {
	data, err := os.ReadFile("testdata/parity-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Upstream  string `json:"upstream"`
		Timestamp string `json:"timestamp"`
		Cases     []struct {
			Name         string   `json:"name"`
			Persistent   Fields   `json:"persistent"`
			Buffer       bool     `json:"buffer"`
			MaxBytes     int      `json:"maxBytes"`
			BufferAt     string   `json:"bufferAt"`
			DisableFlush bool     `json:"disableFlush"`
			Formatter    bool     `json:"formatter"`
			Replacer     string   `json:"replacer"`
			Records      []Fields `json:"records"`
			Snapshots    []Fields `json:"snapshots"`
			Steps        []struct {
				Op      string   `json:"op"`
				Target  string   `json:"target"`
				Name    string   `json:"name"`
				Fields  Fields   `json:"fields"`
				Keys    []string `json:"keys"`
				Value   string   `json:"value"`
				Message string   `json:"message"`
			} `json:"steps"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Upstream != "2.35.0" || len(fixture.Cases) == 0 {
		t.Fatal("missing pinned reference corpus")
	}
	now, err := time.Parse(time.RFC3339Nano, fixture.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("AWS_LAMBDA_MAX_CONCURRENCY", "")
			var output bytes.Buffer
			options := []Option{WithOutput(&output), WithServiceName("orders"), WithPersistentKeys(tc.Persistent), WithClock(func() time.Time { return now })}
			if tc.Buffer {
				level, _ := ParseLevel(tc.BufferAt)
				if tc.BufferAt == "" {
					level = DebugLevel
				}
				options = append(options, WithBuffer(BufferOptions{MaxBytes: tc.MaxBytes, BufferAt: level, DisableFlushOnError: tc.DisableFlush}))
			}
			if tc.Formatter {
				options = append(options, WithFormatter(func(f Fields) (any, error) {
					nested := cloneFields(f)
					for _, key := range []string{"level", "message", "timestamp", "service", "sampling_rate", "xray_trace_id"} {
						delete(nested, key)
					}
					return Fields{"message": f["message"], "empty": "", "null_value": nil, "nested": nested}, nil
				}))
			}
			if tc.Replacer != "" {
				options = append(options, WithReplacer(func(key string, value any) any {
					if tc.Replacer == "root" && key == "" {
						fields := cloneFields(value.(Fields))
						fields["replaced_empty"], fields["replaced_null"] = "", nil
						return fields
					}
					if tc.Replacer == "values" {
						switch key {
						case "null_after":
							return nil
						case "empty_after":
							return ""
						case "empty", "null_value":
							return "RESCUED"
						}
					}
					return value
				}))
			}
			loggers := map[string]*Logger{"parent": New(options...)}
			snapshots := []Fields{}
			for _, step := range tc.Steps {
				target := step.Target
				if target == "" {
					target = "parent"
				}
				l := loggers[target]
				if l == nil {
					t.Fatalf("unknown target %q", target)
				}
				var err error
				switch step.Op {
				case "child":
					loggers[step.Name] = l.Child(WithPersistentKeys(step.Fields))
				case "append":
					l.AppendKeys(step.Fields)
				case "persistent":
					l.AppendPersistentKeys(step.Fields)
				case "remove":
					l.RemoveKeys(step.Keys...)
				case "removePersistent":
					l.RemovePersistentKeys(step.Keys...)
				case "reset":
					l.ResetKeys()
				case "snapshot":
					snapshots = append(snapshots, Fields{"target": target, "persistent": l.PersistentKeys(), "correlation": l.GetCorrelationID()})
				case "trace":
					t.Setenv("_X_AMZN_TRACE_ID", step.Value)
				case "flush":
					err = l.FlushBuffer()
				case "clear":
					l.ClearBuffer()
				default:
					level, ok := ParseLevel(step.Op)
					if !ok {
						t.Fatalf("unknown operation %q", step.Op)
					}
					err = l.Log(level, step.Message, step.Fields)
				}
				if err != nil {
					t.Fatalf("%s: %v", step.Op, err)
				}
			}
			got := records(t, &output)
			if got == nil {
				got = []Fields{}
			}
			for _, record := range got {
				delete(record, "timestamp")
				if value, ok := record["error"].(map[string]any); ok {
					record["error"] = map[string]any{"message": value["message"]}
				}
			}
			if !reflect.DeepEqual(got, tc.Records) {
				t.Fatalf("records\nGo: %#v\nTypeScript: %#v", got, tc.Records)
			}
			// JSON round-trip normalizes named Fields maps and Go numeric types.
			encoded, err := json.Marshal(snapshots)
			if err != nil {
				t.Fatal(err)
			}
			var normalized []Fields
			if err := json.Unmarshal(encoded, &normalized); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalized, tc.Snapshots) {
				t.Fatalf("snapshots\nGo: %#v\nTypeScript: %#v", normalized, tc.Snapshots)
			}
		})
	}
}
