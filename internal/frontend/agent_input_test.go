package frontend

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeRuntimeLineItemsRejectsMalformedSuffixes(t *testing.T) {
	_, err := normalizeRuntimeLineItems("registry name", []byte("ghcr.io\nbad?query=1\n"))
	if err == nil {
		t.Fatalf("expected invalid runtime line error")
	}
	if !strings.Contains(err.Error(), "query or fragment") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSanitizeCommandOutputLinesStripsANSIAndBlankLines(t *testing.T) {
	got := sanitizeCommandOutputLines([]byte("\x1b[31mghcr.io\x1b[0m\n\n docker.io \x00\n"))
	want := []string{"ghcr.io", "docker.io"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sanitized lines: got %#v want %#v", got, want)
	}
}

func TestFieldOrDashHelpersDropInvalidStructuredValues(t *testing.T) {
	if got := fieldOrDashReference("image reference", "docker.io/library/app:latest?download=1"); got != "-" {
		t.Fatalf("expected invalid reference to be dropped, got %q", got)
	}
	if got := fieldOrDashIdentifier("vm name", "\"sandbox\""); got != "-" {
		t.Fatalf("expected quoted identifier to be dropped, got %q", got)
	}
	if got := fieldOrDashPath("workspace path", "/tmp/work#frag"); got != "-" {
		t.Fatalf("expected invalid path to be dropped, got %q", got)
	}
}
