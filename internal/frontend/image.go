package frontend

import "github.com/spf13/cobra"

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Manage OCI images",
}

func init() {
	rootCmd.AddCommand(imageCmd)
}
