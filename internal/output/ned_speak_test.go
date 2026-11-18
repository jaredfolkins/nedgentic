package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatDecoratesOnlyForInteractiveTerminals(t *testing.T) {
	restore := Snapshot()
	t.Cleanup(func() { Configure(restore) })

	Configure(Config{
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
		StdoutIsTTY: true,
		StderrIsTTY: true,
	})
	decorated := Format("hello")
	if !strings.Contains(decorated, "hello") || decorated == "hello" {
		t.Fatalf("expected decorated output, got %q", decorated)
	}

	Configure(Config{
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
		StdoutIsTTY: false,
		StderrIsTTY: true,
	})
	if got := Format("hello"); got != "hello" {
		t.Fatalf("expected plain output when stdout is not a tty, got %q", got)
	}
}

func TestPrintWritesToConfiguredStderr(t *testing.T) {
	restore := Snapshot()
	t.Cleanup(func() { Configure(restore) })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	Configure(Config{
		Stdout:      &stdout,
		Stderr:      &stderr,
		StdoutIsTTY: false,
		StderrIsTTY: false,
	})

	Print("hello")

	if stdout.Len() != 0 {
		t.Fatalf("expected stdout to remain empty, got %q", stdout.String())
	}
	if got := stderr.String(); got != "hello\n" {
		t.Fatalf("unexpected stderr output: %q", got)
	}
}

func TestWriteFormattedLinesDecoratesEachNonEmptyLine(t *testing.T) {
	restore := Snapshot()
	t.Cleanup(func() { Configure(restore) })

	var out bytes.Buffer
	Configure(Config{
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
		StdoutIsTTY: true,
		StderrIsTTY: true,
	})

	WriteFormattedLines(&out, "[0/6] starting\r\n\n[1/6] fetching\n")

	got := out.String()
	if strings.Count(got, "[") < 2 || strings.Contains(got, "\n\n") {
		t.Fatalf("expected decorated non-empty lines only, got %q", got)
	}
	if !strings.Contains(got, "[0/6] starting") || !strings.Contains(got, "[1/6] fetching") {
		t.Fatalf("expected source lines to be preserved, got %q", got)
	}
}
