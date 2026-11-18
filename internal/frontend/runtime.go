package frontend

import "github.com/spf13/cobra"

var flagRuntimeDryRun bool

var runtimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Manage the container runtime",
}

func init() {
	rootCmd.AddCommand(runtimeCmd)
}
