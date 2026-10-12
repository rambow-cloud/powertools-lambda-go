package logger

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type boundaryTextMap map[string]string

func (boundaryTextMap) MarshalText() ([]byte, error) { return []byte("map text"), nil }

type boundaryTextSlice []string

func (boundaryTextSlice) MarshalText() ([]byte, error) { return []byte("slice text"), nil }

type boundaryPointerText string

func (*boundaryPointerText) MarshalText() ([]byte, error) { return []byte("pointer text"), nil }

type boundaryFailTextMap map[string]string

var boundaryMarshalError = errors.New("boundary marshaler failed")

func (boundaryFailTextMap) MarshalText() ([]byte, error) { return nil, boundaryMarshalError }

type boundaryCountMarshaler struct{ calls int }

func (m *boundaryCountMarshaler) MarshalJSON() ([]byte, error) {
	m.calls++
	return nil, boundaryMarshalError
}

func TestLoggerConfigurationBoundaryReference(t *testing.T) {
	data, err := os.ReadFile("testdata/boundaries-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string `json:"version"`
		Cases   []struct {
			Name  string            `json:"name"`
			Now   string            `json:"now"`
			Env   map[string]string `json:"env"`
			Level string            `json:"level"`
			Error *struct {
				Name    string `json:"name"`
				Message string `json:"message"`
			} `json:"error"`
			Records []struct {
				Sink   string `json:"sink"`
				Record Fields `json:"record"`
			} `json:"records"`
			Diagnostics []string `json:"diagnostics"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "2.35.0" || len(fixture.Cases) != 26 {
		t.Fatal("unexpected configuration corpus")
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			cleanEnv(t)
			for key, value := range row.Env {
				t.Setenv(key, value)
			}
			now, err := time.Parse(time.RFC3339Nano, row.Now)
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			log := New(WithServiceName("audit"), WithClock(func() time.Time { return now }), func(c *config) { c.stdout = &stdout; c.stderr = &stderr })
			for _, level := range []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel} {
				if err := log.Log(level, strings.ToLower(level.String())); err != nil {
					t.Fatal(err)
				}
			}
			got := map[string][]Fields{"stdout": {}, "stderr": {}}
			var diagnostics []string
			for sink, buffer := range map[string]*bytes.Buffer{"stdout": &stdout, "stderr": &stderr} {
				for _, record := range records(t, buffer) {
					if message, ok := record["message"].(string); ok && strings.HasPrefix(message, "Invalid or unresolvable time zone:") {
						diagnostics = append(diagnostics, message)
						continue
					}
					got[sink] = append(got[sink], record)
				}
			}
			if row.Error != nil {
				// The stable New API deliberately falls back; TypeScript throws for
				// a nonnumeric sample-rate environment variable. Preserve that evidence.
				if row.Name != "invalid-rate" || row.Error.Name != "TypeError" || log.GetLevelName() != "INFO" || len(got["stdout"]) != 1 || len(got["stderr"]) != 3 {
					t.Fatalf("fallback=%v error=%v", got, row.Error)
				}
				for _, list := range got {
					for _, record := range list {
						if record["sampling_rate"] != float64(0) {
							t.Fatal(record)
						}
					}
				}
				return
			}
			want := map[string][]Fields{"stdout": {}, "stderr": {}}
			for _, record := range row.Records {
				expected := cloneFields(record.Record)
				if row.Name == "Europe/London-2026-01-01T00:00:00.123Z" || strings.HasPrefix(row.Name, "Not/AZone-") {
					// Intl uses +00:00 for these explicit/fallback zones; Go's
					// RFC3339 layout uses Z. Keep this finite native difference explicit.
					if expected["timestamp"] != strings.TrimSuffix(row.Now, "Z")+"+00:00" {
						t.Fatal("unexpected upstream zero-offset representation")
					}
					expected["timestamp"] = row.Now
				}
				want[record.Sink] = append(want[record.Sink], expected)
			}
			if !reflect.DeepEqual(got, want) || log.GetLevelName() != row.Level {
				t.Fatalf("records=%v level=%s want=%v %s", got, log.GetLevelName(), want, row.Level)
			}
			if len(diagnostics) != len(row.Diagnostics) {
				t.Fatalf("diagnostics=%v want=%v", diagnostics, row.Diagnostics)
			}
			for i, diagnostic := range diagnostics {
				if diagnostic != row.Diagnostics[i] {
					t.Fatalf("diagnostic=%q want=%q", diagnostic, row.Diagnostics[i])
				}
			}
		})
	}
}

func TestNativeTextMarshalerContainers(t *testing.T) {
	cleanEnv(t)
	text := boundaryPointerText("underlying")
	for _, replace := range []bool{false, true} {
		for _, row := range []struct {
			name    string
			value   any
			want    string
			wantErr error
		}{
			{"map", boundaryTextMap{"underlying": "value"}, "map text", nil},
			{"slice", boundaryTextSlice{"underlying"}, "slice text", nil},
			{"pointer", &text, "pointer text", nil},
			{"error", boundaryFailTextMap{"underlying": "value"}, "", boundaryMarshalError},
		} {
			t.Run(row.name+map[bool]string{true: "-replacer", false: "-plain"}[replace], func(t *testing.T) {
				var output bytes.Buffer
				options := []Option{WithOutput(&output)}
				calls := 0
				if replace {
					options = append(options, WithReplacer(func(key string, value any) any {
						if key == "payload" {
							calls++
						}
						return value
					}))
				}
				log := New(options...)
				err := log.Info("native", Fields{"payload": row.value})
				if row.wantErr != nil {
					var semantic *json.SemanticError
					if !errors.Is(err, row.wantErr) || !errors.As(err, &semantic) || output.Len() != 0 {
						t.Fatalf("error=%v output=%s", err, &output)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if got := records(t, &output)[0]["payload"]; got != row.want {
						t.Fatalf("payload=%v want=%s", got, row.want)
					}
				}
				if replace && calls != 1 {
					t.Fatalf("replacer calls=%d", calls)
				}
			})
		}
	}
	t.Run("formatter", func(t *testing.T) {
		var output bytes.Buffer
		log := New(WithOutput(&output), WithFormatter(func(Fields) (any, error) { return boundaryTextMap{"empty": ""}, nil }))
		if err := log.Info("native"); err != nil {
			t.Fatal(err)
		}
		if output.String() != `"map text"`+"\n" {
			t.Fatal(output.String())
		}
	})
}

func TestFailedJSONNormalizationDoesNotRetryMarshaler(t *testing.T) {
	cleanEnv(t)
	for _, streaming := range []bool{false, true} {
		var output bytes.Buffer
		value := &boundaryCountMarshaler{}
		var payload any = value
		stream := &boundaryStreamingMarshaler{}
		if streaming {
			payload = stream
		}
		log := New(WithOutput(&output), WithReplacer(func(_ string, value any) any { return value }))
		err := log.Info("native", Fields{"nested": []any{Fields{"payload": payload}}})
		calls := value.calls
		if streaming {
			calls = stream.calls
		}
		var semantic *json.SemanticError
		if !errors.Is(err, boundaryMarshalError) || !errors.As(err, &semantic) || calls != 1 || output.Len() != 0 {
			t.Fatalf("streaming=%v calls=%d output=%s error=%v", streaming, calls, &output, err)
		}
	}
}

type boundaryStreamingMarshaler struct{ calls int }

func (m *boundaryStreamingMarshaler) MarshalJSONTo(*jsontext.Encoder) error {
	m.calls++
	return boundaryMarshalError
}

type boundaryShortWriter struct{ calls int }

func (w *boundaryShortWriter) Write(data []byte) (int, error) { w.calls++; return len(data) - 1, nil }
func TestSinkShortWriteAndChildRoute(t *testing.T) {
	cleanEnv(t)
	short := &boundaryShortWriter{}
	root := New(WithOutput(short))
	if err := root.Info("short"); !errors.Is(err, io.ErrShortWrite) || short.calls != 1 {
		t.Fatalf("calls=%d error=%v", short.calls, err)
	}
	var parent, child bytes.Buffer
	root = New(WithOutput(&parent))
	nested := root.Child(WithOutput(&child))
	if err := root.Info("parent"); err != nil {
		t.Fatal(err)
	}
	if err := nested.Warn("child"); err != nil {
		t.Fatal(err)
	}
	if len(records(t, &parent)) != 1 || len(records(t, &child)) != 1 || records(t, &parent)[0]["message"] != "parent" || records(t, &child)[0]["message"] != "child" {
		t.Fatalf("parent=%s child=%s", &parent, &child)
	}
}
