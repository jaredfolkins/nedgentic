package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/output"
)

var profileInstallOverwrite bool

var profileInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the default personal profile",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureProfilesInstalled(profileInstallOverwrite, true); err != nil {
			output.Printf("Error installing profiles: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	profileInstallCmd.Flags().BoolVarP(&profileInstallOverwrite, "overwrite", "o", false, "Overwrite the default personal profile")
	profileCmd.AddCommand(profileInstallCmd)
}
