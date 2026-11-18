package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var imagePullCmd = &cobra.Command{
	Use:   "pull <ref>",
	Short: "Pull an OCI image",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		imageRef, err := normalizeAgentReference("image reference", args[0])
		if err != nil {
			output.Printf("Error pulling image: %v", err)
			os.Exit(1)
		}

		flags := profile.DefaultFlags()
		flags = applyFlagOverrides(flags, cmd)
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))

		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}

		output.Printf("Pulling image %s...", imageRef)
		if err := ensureImagePull(bin, imageRef, flags.Platform); err != nil {
			output.Printf("Error pulling image: %v", err)
			os.Exit(1)
		}
		output.Print("Image pulled successfully.")
	},
}

func init() {
	imageCmd.AddCommand(imagePullCmd)
}
