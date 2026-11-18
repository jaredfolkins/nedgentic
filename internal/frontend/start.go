package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var startCmd = &cobra.Command{
	Use:   "start [vm]...",
	Short: "Start stopped VMs without attaching",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags := profile.DefaultFlags()
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))
		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		var failed bool
		for _, raw := range args {
			name, err := normalizeAgentName("vm name", raw)
			if err != nil {
				output.Printf("Start failed: %v", err)
				failed = true
				continue
			}
			exists, status, err := runtime.ContainerStatusByName(bin, name)
			if err != nil {
				output.Printf("Error reading VM status for %s: %v", name, err)
				failed = true
				continue
			}
			if !exists {
				output.Printf("VM %s not found", name)
				failed = true
				continue
			}
			if isRunningStatus(status) {
				output.Printf("VM %s already running", name)
				continue
			}
			if err := runtime.ContainerStart(bin, name); err != nil {
				output.Printf("Error starting VM %s: %v", name, err)
				failed = true
			}
		}
		if failed {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
