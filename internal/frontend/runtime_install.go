package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

var flagRuntimeSkipBrew bool

var runtimeInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the runtime on the host",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		opts := runtime.RuntimeActionOptions{
			DryRun:   flagRuntimeDryRun,
			SkipBrew: flagRuntimeSkipBrew,
		}
		if err := runtime.InstallContainerRuntime(cmd.OutOrStdout(), opts); err != nil {
			output.Printf("Runtime install failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	runtimeInstallCmd.Flags().BoolVar(&flagRuntimeDryRun, "dry-run", false, "Print actions without executing them")
	runtimeInstallCmd.Flags().BoolVar(&flagRuntimeSkipBrew, "skip-brew", false, "Skip Homebrew and use the signed pkg installer")
	runtimeCmd.AddCommand(runtimeInstallCmd)
}
