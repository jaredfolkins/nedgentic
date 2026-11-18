package frontend

import (
	"bytes"
	"io"
	"strings"
	"testing"

	backendruntime "nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

func TestConfirmMutationFailsFastWhenNotInteractive(t *testing.T) {
	origTTY := confirmIsTTY
	defer func() {
		confirmIsTTY = origTTY
	}()

	confirmIsTTY = func() bool { return false }
	confirmed, err := confirmMutation("Proceed?", false)
	if err == nil {
		t.Fatalf("expected non-interactive confirmation error")
	}
	if confirmed {
		t.Fatalf("expected confirmation to be false on error")
	}

	confirmed, err = confirmMutation("Proceed?", true)
	if err != nil {
		t.Fatalf("expected --yes to bypass prompt: %v", err)
	}
	if !confirmed {
		t.Fatalf("expected confirmation when --yes is set")
	}
}

func TestRuntimeRestartDryRunTextUsesRuntimeHelpers(t *testing.T) {
	origStop := backendruntime.StopContainerRuntime
	origStart := backendruntime.StartContainerRuntime
	origDryRun := flagRuntimeDryRun
	origYes := flagRuntimeRestartYes
	defer func() {
		backendruntime.StopContainerRuntime = origStop
		backendruntime.StartContainerRuntime = origStart
		flagRuntimeDryRun = origDryRun
		flagRuntimeRestartYes = origYes
	}()

	restore := output.Snapshot()
	t.Cleanup(func() { output.Configure(restore) })
	output.Configure(output.Config{
		Mode:        output.ModeText,
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
		StdoutIsTTY: false,
		StderrIsTTY: false,
	})

	var stopCalled, startCalled bool
	backendruntime.StopContainerRuntime = func(out io.Writer, bin string, opts backendruntime.RuntimeActionOptions) error {
		stopCalled = true
		if !opts.DryRun {
			t.Fatalf("expected stop helper dry-run")
		}
		_, _ = out.Write([]byte("DRY RUN: container system stop\n"))
		return nil
	}
	backendruntime.StartContainerRuntime = func(out io.Writer, bin string, opts backendruntime.RuntimeActionOptions) error {
		startCalled = true
		if !opts.DryRun {
			t.Fatalf("expected start helper dry-run")
		}
		_, _ = out.Write([]byte("DRY RUN: container system start\n"))
		return nil
	}

	flagRuntimeDryRun = true
	flagRuntimeRestartYes = false

	var out bytes.Buffer
	runtimeRestartCmd.SetOut(&out)
	runtimeRestartCmd.Run(runtimeRestartCmd, nil)

	if !stopCalled || !startCalled {
		t.Fatalf("expected runtime restart dry-run to call both helpers, stop=%v start=%v", stopCalled, startCalled)
	}
	if !strings.Contains(out.String(), "DRY RUN: container system stop") {
		t.Fatalf("expected stop dry-run output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "DRY RUN: container system start") {
		t.Fatalf("expected start dry-run output, got %q", out.String())
	}
}
