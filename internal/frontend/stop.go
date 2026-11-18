package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var stopCmd = &cobra.Command{
	Use:   "stop [vm]",
	Short: "Stop a running VM",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, err := normalizeAgentName("vm name", args[0])
		if err != nil {
			output.Printf("Stop failed: %v", err)
			os.Exit(1)
		}
		flags := profile.DefaultFlags()
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))
		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		exists, status, err := runtime.ContainerStatusByName(bin, name)
		if err != nil {
			output.Printf("Error reading VM status for %s: %v", name, err)
			os.Exit(1)
		}
		if !exists {
			output.Printf("VM %s not found", name)
			os.Exit(1)
		}
		if !isRunningStatus(status) {
			output.Printf("VM %s already stopped", name)
			return
		}
		if err := runtime.ContainerStop(bin, name); err != nil {
			output.Printf("Error stopping VM: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
