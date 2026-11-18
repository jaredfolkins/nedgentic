package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
)

var flagRuntimeRestartYes bool

var runtimeRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the container runtime on the host",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !flagRuntimeRestartYes && !flagRuntimeDryRun {
			output.Print("WARNING: Restarting the container runtime will STOP ALL running VMs.")
			confirmed, err := confirmMutation("Are you sure you want to proceed?", flagRuntimeRestartYes)
			if err != nil {
				output.Printf("Runtime restart failed: %v", err)
				os.Exit(1)
			}
			if !confirmed {
				output.Print("Aborted.")
				os.Exit(0)
			}
		}

		opts := runtime.RuntimeActionOptions{
			DryRun: flagRuntimeDryRun,
		}

		// Stop
		if err := runtime.StopContainerRuntime(cmd.OutOrStdout(), "", opts); err != nil {
			output.Printf("Runtime restart (stop phase) failed: %v", err)
			os.Exit(1)
		}

		// Start
		if err := runtime.StartContainerRuntime(cmd.OutOrStdout(), "", opts); err != nil {
			output.Printf("Runtime restart (start phase) failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	runtimeRestartCmd.Flags().BoolVar(&flagRuntimeDryRun, "dry-run", false, "Print actions without executing them")
	runtimeRestartCmd.Flags().BoolVarP(&flagRuntimeRestartYes, "yes", "y", false, "Skip confirmation prompt")
	runtimeCmd.AddCommand(runtimeRestartCmd)
}
