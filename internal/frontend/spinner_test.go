package frontend

import (
	"bytes"
	"testing"

	runtimebackend "nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

func TestRunWithSpinnerStepSuppressesNonTTYFinalOutput(t *testing.T) {
	origWantsTTY := runtimebackend.WantsTTY
	runtimebackend.WantsTTY = func() bool { return false }
	defer func() {
		runtimebackend.WantsTTY = origWantsTTY
	}()

	prev := output.Snapshot()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output.Configure(output.Config{
		Stdout:      &stdout,
		Stderr:      &stderr,
		StdoutIsTTY: false,
		StderrIsTTY: false,
	})
	defer output.Configure(prev)

	if err := runWithSpinnerStep("Wait for ready", func() error { return nil }); err != nil {
		t.Fatalf("runWithSpinnerStep error: %v", err)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected no stderr output, got %q", got)
	}
}

func TestRunWithSpinnerKeepsNonTTYFinalOutput(t *testing.T) {
	origWantsTTY := runtimebackend.WantsTTY
	runtimebackend.WantsTTY = func() bool { return false }
	defer func() {
		runtimebackend.WantsTTY = origWantsTTY
	}()

	prev := output.Snapshot()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output.Configure(output.Config{
		Stdout:      &stdout,
		Stderr:      &stderr,
		StdoutIsTTY: false,
		StderrIsTTY: false,
	})
	defer output.Configure(prev)

	if err := runWithSpinner("Remove VM", func() error { return nil }); err != nil {
		t.Fatalf("runWithSpinner error: %v", err)
	}
	if got := stderr.String(); got != "Remove VM... done\n" {
		t.Fatalf("unexpected stderr output: %q", got)
	}
}
