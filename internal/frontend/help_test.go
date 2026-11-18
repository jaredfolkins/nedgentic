package frontend

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldRenderVerboseHelp(t *testing.T) {
	originalBrevity := flagBrevity
	t.Cleanup(func() {
		flagBrevity = originalBrevity
	})

	tests := []struct {
		name    string
		brevity bool
		want    bool
	}{
		{
			name:    "verbose is default",
			brevity: false,
			want:    true,
		},
		{
			name:    "brevity opts back into concise help",
			brevity: true,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagBrevity = tt.brevity
			if got := shouldRenderVerboseHelp(); got != tt.want {
				t.Fatalf("shouldRenderVerboseHelp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteHelpRootBasicLayout(t *testing.T) {
	var out bytes.Buffer
	writeHelp(&out, rootCmd, false)
	rendered := out.String()

	if !strings.Contains(rendered, "Usage:\n  ned [command]\n") {
		t.Fatalf("expected concise usage, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Common Commands:") {
		t.Fatalf("expected common commands section, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "gentic") {
		t.Fatalf("expected gentic command summary, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Common Examples:") {
		t.Fatalf("expected common examples section, got:\n%s", rendered)
	}
	wantExamplesBlock := "Common Examples:\n" +
		"  ned profile install --overwrite\n" +
		"  ned gentic personal default\n" +
		"  ned exec <vm> -- /bin/bash\n" +
		"  ned runtime status\n"
	if !strings.Contains(rendered, wantExamplesBlock) {
		t.Fatalf("expected stable examples block, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "Flags:") {
		t.Fatalf("expected basic root help to omit full flags list, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "ned help --verbose") {
		t.Fatalf("expected old verbose hint to be removed, got:\n%s", rendered)
	}
}

func TestWriteHelpSubcommandUsesUsageString(t *testing.T) {
	var out bytes.Buffer
	writeHelp(&out, runtimeCmd, false)
	rendered := out.String()

	if !strings.Contains(rendered, "Usage:") {
		t.Fatalf("expected usage output for subcommand, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Use ned [command] --help without --brevity to show the full command tree") {
		t.Fatalf("expected brevity hint for subcommand, got:\n%s", rendered)
	}
}

func TestWriteHelpVerboseRendersCommandTreeFromRootOrSubcommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{
			name: "root command",
			cmd:  rootCmd,
		},
		{
			name: "subcommand",
			cmd:  runtimeCmd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			writeHelp(&out, tt.cmd, true)
			rendered := out.String()

			if !strings.HasPrefix(rendered, "Verbose command tree:\n") {
				t.Fatalf("expected verbose command tree header, got:\n%s", rendered)
			}
			if !strings.Contains(rendered, "ned — Ned: CLI for Nedgentic") {
				t.Fatalf("expected root command in verbose output, got:\n%s", rendered)
			}
			if !strings.Contains(rendered, "runtime — Manage the container runtime") {
				t.Fatalf("expected runtime command in verbose output, got:\n%s", rendered)
			}
			if !strings.Contains(rendered, "Global flags:\n") {
				t.Fatalf("expected global flags section in verbose output, got:\n%s", rendered)
			}
			if !strings.Contains(rendered, "--log-level: Log level (debug|info|warn|error)") {
				t.Fatalf("expected global flag details in verbose output, got:\n%s", rendered)
			}
		})
	}
}

func TestRenderCommandTreeDeduplicatesInheritedFlags(t *testing.T) {
	rendered := renderCommandTree(rootCmd)

	if strings.Count(rendered, "--log-level: Log level (debug|info|warn|error)") != 1 {
		t.Fatalf("expected global --log-level flag once, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "  list — List Nedgentic VMs\n    --all: Include stopped VMs\n    --output: Output format (text or json)\n") {
		t.Fatalf("expected list command to render its own flags, got:\n%s", rendered)
	}
}

func TestSkillsCommandIsRemoved(t *testing.T) {
	if strings.Contains(renderCommandTree(rootCmd), "skills") {
		t.Fatal("ned must not offer a skills command; agents manage their own skills inside the VM")
	}
}

func TestAgentIntrospectionCommandsAreRemoved(t *testing.T) {
	for _, name := range []string{"mcp", "schema", "agent"} {
		if findNamedSubcommand(rootCmd, name) != nil {
			t.Fatalf("ned must not offer `ned %s`", name)
		}
	}
	if newHelpCmd().Flags().Lookup("json") != nil {
		t.Fatal("ned help must not offer --json")
	}
	for _, name := range []string{"fields", "limit", "no-summary"} {
		if listCmd.Flags().Lookup(name) != nil {
			t.Fatalf("ned list must not offer --%s", name)
		}
	}
	if stopCmd.Flags().Lookup("dry-run") != nil {
		t.Fatal("ned stop must not offer --dry-run")
	}
}

func TestBrevityFlagIsRegistered(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("brevity")
	if flag == nil {
		t.Fatal("expected --brevity flag to be registered")
	}
	if rootCmd.PersistentFlags().Lookup("verbose") != nil {
		t.Fatal("expected --verbose flag to be removed")
	}
}
