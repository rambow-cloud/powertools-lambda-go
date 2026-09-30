package xray

import (
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	sdk "github.com/aws/aws-xray-sdk-go/v2/xray"
)

func TestConstructorDeprecationWarningOnce(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = writer
	warnOnce = sync.Once{}
	t.Cleanup(func() {
		os.Stderr = original
		_ = writer.Close()
		_ = reader.Close()
		warnOnce = sync.Once{}
	})
	New(sdk.Config{})
	New(sdk.Config{})
	_ = writer.Close()
	os.Stderr = original
	output, err := io.ReadAll(reader)
	if err != nil || string(output) != DeprecationWarning+"\n" || !strings.Contains(string(output), "OpenTelemetry") {
		t.Fatalf("migration warning: %q, error: %v", output, err)
	}
}
