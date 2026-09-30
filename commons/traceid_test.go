package commons_test

import (
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func TestFormatXRayTraceID(t *testing.T) {
	for input, expected := range map[string]string{
		"65abcdef123456789012345678901234": "1-65abcdef-123456789012345678901234",
		"65ABCDEF123456789012345678901234": "1-65abcdef-123456789012345678901234",
		"00000000000000000000000000000000": "",
		"65abcdef12345678901234567890123z": "",
		"":                                 "", "1234": "",
	} {
		if got := commons.FormatXRayTraceID(input); got != expected {
			t.Fatalf("%q: got %q, want %q", input, got, expected)
		}
	}
}
