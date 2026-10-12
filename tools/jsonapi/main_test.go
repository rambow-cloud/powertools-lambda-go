package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectSource(t *testing.T) {
	for _, test := range []struct {
		name, source string
		failures     int
	}{
		{"default import", `package p; import "encoding/json"; var _ = json.Unmarshal`, 1},
		{"aliased import", `package p; import old "encoding/json"; var _ = old.NewDecoder`, 1},
		{"type interoperability", `package p; import old "encoding/json"; var _ old.Number; var _ old.RawMessage`, 0},
		{"legacy options", `package p; import old "encoding/json"; var _ = old.DefaultOptionsV1`, 1},
		{"legacy interface", `package p; import old "encoding/json"; var _ old.Marshaler`, 1},
		{"dot import", `package p; import . "encoding/json"; var _ = Unmarshal`, 1},
		{"v2", `package p; import "encoding/json/v2"; var _ = json.Unmarshal`, 0},
		{"shadowed local", `package p; import "encoding/json"; var _ json.Number; func f() { json := struct{ Marshal int }{}; _ = json.Marshal }`, 0},
		{"comments and strings", "package p; // json.Unmarshal\n var _ = `encoding/json.NewEncoder`", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			failures, err := inspectSource("example.go", test.source)
			if err != nil || len(failures) != test.failures {
				t.Fatalf("failures=%v, err=%v", failures, err)
			}
		})
	}
}

func TestInspectTree(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"first_test.go", "dist/generated.go", "node_modules/dependency.go", "vendor/external.go"} {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`package p; import "encoding/json"; var _ = json.Marshal`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	failures, err := inspectTree(root)
	if err != nil || len(failures) != 1 || !strings.Contains(failures[0], "first_test.go") {
		t.Fatalf("failures=%v, err=%v", failures, err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid.go"), []byte("invalid Go"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectTree(root); err == nil {
		t.Fatal("invalid Go source was silently skipped")
	}
}
