package frontend

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

func TestResolveRuntimeStatusContainerBinRejectsEmptyPath(t *testing.T) {
	origResolve := runtimeStatusResolveContainerBin
	defer func() {
		runtimeStatusResolveContainerBin = origResolve
	}()

	runtimeStatusResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "", nil
	}

	_, err := resolveRuntimeStatusContainerBin(&cobra.Command{Use: "test"})
	if err == nil {
		t.Fatal("expected error for empty bin")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRuntimeStatusCommandFlagWiring(t *testing.T) {
	if runtimeStatusCmd.Flags().Lookup("format") == nil {
		t.Fatalf("expected --format flag on runtime status command")
	}
}

func TestRuntimeStatusCommandRunWithFormatInvokesSystemStatus(t *testing.T) {
	origFormat := runtimeStatusFormat
	origResolve := runtimeStatusResolveContainerBin
	origSystemStatus := runtimeStatusSystemStatus
	origWrite := runtimeStatusWriteRuntimeStatus
	defer func() {
		runtimeStatusFormat = origFormat
		runtimeStatusResolveContainerBin = origResolve
		runtimeStatusSystemStatus = origSystemStatus
		runtimeStatusWriteRuntimeStatus = origWrite
	}()

	runtimeStatusResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "/usr/local/bin/container", nil
	}

	var gotBin string
	var gotFormat string
	runtimeStatusSystemStatus = func(bin, format string) ([]byte, error) {
		gotBin = bin
		gotFormat = format
		return []byte(`{"status":"running"}`), nil
	}

	writeCalled := false
	runtimeStatusWriteRuntimeStatus = func(w io.Writer) error {
		writeCalled = true
		return nil
	}

	runtimeStatusFormat = "json"
	runtimeStatusCmd.Run(runtimeStatusCmd, nil)

	if writeCalled {
		t.Fatalf("did not expect legacy runtime status writer to be called when --format is set")
	}
	if gotBin != "/usr/local/bin/container" {
		t.Fatalf("unexpected command binary: %q", gotBin)
	}
	if gotFormat != "json" {
		t.Fatalf("unexpected format: %q", gotFormat)
	}
}

func TestRuntimeStatusCommandRunWithoutFormatUsesLegacyWriter(t *testing.T) {
	origFormat := runtimeStatusFormat
	origSystemStatus := runtimeStatusSystemStatus
	origWrite := runtimeStatusWriteRuntimeStatus
	defer func() {
		runtimeStatusFormat = origFormat
		runtimeStatusSystemStatus = origSystemStatus
		runtimeStatusWriteRuntimeStatus = origWrite
	}()

	systemStatusCalled := false
	runtimeStatusSystemStatus = func(bin, format string) ([]byte, error) {
		systemStatusCalled = true
		return nil, nil
	}

	writeCalled := false
	runtimeStatusWriteRuntimeStatus = func(w io.Writer) error {
		writeCalled = true
		_, _ = io.WriteString(w, "container path: /usr/local/bin/container\n")
		return nil
	}

	runtimeStatusFormat = ""
	runtimeStatusCmd.Run(runtimeStatusCmd, nil)

	if !writeCalled {
		t.Fatalf("expected legacy runtime status writer to be called")
	}
	if systemStatusCalled {
		t.Fatalf("did not expect system status command to be called without --format")
	}
}

func TestRuntimeStatusCommandUsesStructuredEnvelopeWhenGlobalOutputIsJSON(t *testing.T) {
	restoreOutput := output.Snapshot()
	origFormat := runtimeStatusFormat
	origCollect := runtimeStatusCollectRuntimeStatus
	origSystemStatus := runtimeStatusSystemStatus
	origWrite := runtimeStatusWriteRuntimeStatus
	defer func() {
		output.Configure(restoreOutput)
		runtimeStatusFormat = origFormat
		runtimeStatusCollectRuntimeStatus = origCollect
		runtimeStatusSystemStatus = origSystemStatus
		runtimeStatusWriteRuntimeStatus = origWrite
	}()

	output.Configure(output.Config{
		Mode:        output.ModeJSON,
		Stdout:      &bytes.Buffer{},
		Stderr:      &bytes.Buffer{},
		StdoutIsTTY: false,
		StderrIsTTY: false,
	})
	runtimeStatusFormat = "table"
	runtimeStatusCollectRuntimeStatus = func() runtime.RuntimeStatus {
		return runtime.RuntimeStatus{ContainerPath: "/usr/local/bin/container"}
	}
	runtimeStatusSystemStatus = func(bin, format string) ([]byte, error) {
		t.Fatalf("did not expect raw runtime status path in structured mode")
		return nil, nil
	}
	runtimeStatusWriteRuntimeStatus = func(w io.Writer) error {
		t.Fatalf("did not expect legacy writer in structured mode")
		return nil
	}

	cmd := &cobra.Command{Use: "status"}
	var out bytes.Buffer
	cmd.SetOut(&out)
	runtimeStatusCmd.Run(cmd, nil)

	if !strings.Contains(out.String(), "\"kind\": \"runtime_status\"") {
		t.Fatalf("expected structured runtime status output, got %q", out.String())
	}
}
