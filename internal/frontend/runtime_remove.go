package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

var runtimeRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the container runtime from the host",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		opts := runtime.RuntimeActionOptions{DryRun: flagRuntimeDryRun}
		if err := runtime.RemoveContainerRuntime(cmd.OutOrStdout(), opts); err != nil {
			output.Printf("Runtime remove failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	runtimeRemoveCmd.Flags().BoolVar(&flagRuntimeDryRun, "dry-run", false, "Print actions without executing them")
	runtimeCmd.AddCommand(runtimeRemoveCmd)
}
