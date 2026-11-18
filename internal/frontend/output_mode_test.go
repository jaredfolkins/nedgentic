package frontend

import (
	"strings"
	"testing"

	"nedgentic/internal/output"
)

func TestConfigureOutputUsesOutputFlag(t *testing.T) {
	restore := output.Snapshot()
	origOutput := flagOutput
	t.Cleanup(func() {
		output.Configure(restore)
		flagOutput = origOutput
	})

	flagOutput = "json"
	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput error: %v", err)
	}
	if got := output.CurrentMode(); got != output.ModeJSON {
		t.Fatalf("unexpected mode: %q", got)
	}

	flagOutput = "ndjson"
	if err := configureOutput(); err == nil {
		t.Fatalf("expected ndjson to be rejected")
	}
}

func TestOutputFlagOnlyOnReadCommands(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup(outputFlag) != nil {
		t.Fatalf("--output must not be a global flag")
	}
	for _, cmd := range []string{"list", "image list", "profile list", "staging list", "storage list", "runtime status"} {
		target, err := resolveCommandPath(rootCmd, strings.Fields(cmd))
		if err != nil {
			t.Fatalf("resolve %s: %v", cmd, err)
		}
		if target.Flags().Lookup(outputFlag) == nil {
			t.Fatalf("%s should accept --output", cmd)
		}
	}
}
