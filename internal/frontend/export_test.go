package frontend

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"nedgentic/internal/profile"
)

func TestResolveExportContainerBinRejectsEmptyPath(t *testing.T) {
	origResolve := exportResolveContainerBin
	defer func() {
		exportResolveContainerBin = origResolve
	}()

	exportResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "", nil
	}

	_, err := resolveExportContainerBin(&cobra.Command{Use: "test"})
	if err == nil {
		t.Fatal("expected error for empty bin")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExportCommandFlagWiring(t *testing.T) {
	if exportCmd.Flags().Lookup("output") == nil {
		t.Fatalf("expected --output flag on export command")
	}
	if exportCmd.Flags().ShorthandLookup("o") == nil {
		t.Fatalf("expected -o shorthand on export command")
	}
	if exportCmd.Flags().Lookup("image") != nil {
		t.Fatalf("did not expect legacy --image flag on export command")
	}
}

func TestResolveExportOutputPathDefaultsToStdout(t *testing.T) {
	got, err := resolveExportOutputPath(&cobra.Command{Use: "test"}, "/tmp/out.tar")
	if err != nil {
		t.Fatalf("resolveExportOutputPath error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty default output path, got %q", got)
	}
}

func TestResolveExportOutputPathNormalizesPath(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("output", "", "")
	if err := cmd.Flags().Set("output", "/tmp/out.tar"); err != nil {
		t.Fatalf("set output flag: %v", err)
	}

	got, err := resolveExportOutputPath(cmd, "/tmp/out.tar")
	if err != nil {
		t.Fatalf("resolveExportOutputPath error: %v", err)
	}
	if got != "/tmp/out.tar" {
		t.Fatalf("unexpected output path: %q", got)
	}
}

func TestResolveExportOutputPathRejectsURL(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("output", "", "")
	if err := cmd.Flags().Set("output", "https://example.com/out.tar"); err != nil {
		t.Fatalf("set output flag: %v", err)
	}

	_, err := resolveExportOutputPath(cmd, "https://example.com/out.tar")
	if err == nil {
		t.Fatal("expected error for URL output path")
	}
	if !strings.Contains(err.Error(), "filesystem path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExportCommandRunForwardsArgs(t *testing.T) {
	origResolve := exportResolveContainerBin
	origExport := exportContainer
	origOutput := exportOutput
	outputFlag := exportCmd.Flags().Lookup("output")
	origChanged := false
	if outputFlag != nil {
		origChanged = outputFlag.Changed
	}
	defer func() {
		exportResolveContainerBin = origResolve
		exportContainer = origExport
		exportOutput = origOutput
		if outputFlag != nil {
			_ = outputFlag.Value.Set(origOutput)
			outputFlag.Changed = origChanged
		}
	}()

	exportResolveContainerBin = func(flags profile.Flags) (string, error) {
		return "/usr/local/bin/container", nil
	}

	var gotBin string
	var gotID string
	var gotOutput string
	exportContainer = func(bin, id, output string) error {
		gotBin = bin
		gotID = id
		gotOutput = output
		return nil
	}

	if err := exportCmd.Flags().Set("output", "/tmp/sandbox.tar"); err != nil {
		t.Fatalf("set output flag: %v", err)
	}
	exportCmd.Run(exportCmd, []string{"sandbox-vm"})

	if gotBin != "/usr/local/bin/container" {
		t.Fatalf("unexpected command binary: %q", gotBin)
	}
	if gotID != "sandbox-vm" {
		t.Fatalf("unexpected container ID: %q", gotID)
	}
	if gotOutput != "/tmp/sandbox.tar" {
		t.Fatalf("unexpected output path: %q", gotOutput)
	}
}
