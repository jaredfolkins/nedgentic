package frontend

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var removeCmd = &cobra.Command{
	Use:     "remove [vm]...",
	Aliases: []string{"rm"},
	Short:   "Stop and delete VMs",
	Args:    cobra.MinimumNArgs(1),
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
				output.Printf("Remove failed: %v", err)
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
			if shouldStopContainer(status) {
				if err := runWithSpinner(fmt.Sprintf("Stop VM: %s", name), func() error {
					return runtime.ContainerStop(bin, name)
				}); err != nil {
					output.Printf("Error stopping VM %s: %v", name, err)
					failed = true
				}
			}
			if err := runWithSpinner(fmt.Sprintf("Delete VM: %s", name), func() error {
				return runtime.ContainerDelete(bin, name)
			}); err != nil {
				output.Printf("Error deleting VM %s: %v", name, err)
				failed = true
			}
			if err := runWithSpinner(fmt.Sprintf("Delete agent logins: %s", name), func() error {
				return runtime.DeleteAgentAuthVolume(bin, name)
			}); err != nil {
				output.Printf("Error deleting agent logins for %s: %v", name, err)
				failed = true
			}
			if err := runtime.RemoveStagingDir(name); err != nil {
				output.Printf("Error deleting staging for %s: %v", name, err)
				failed = true
			}
		}
		if failed {
			os.Exit(1)
		}
	},
}

func shouldStopContainer(status string) bool {
	status = strings.TrimSpace(status)
	return strings.EqualFold(status, "running")
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
