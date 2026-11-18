package frontend

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var imageListTableFields = []tableField[runtime.ImageListEntry]{
	{Header: "REPOSITORY", Value: func(item runtime.ImageListEntry) string { return item.Repository }},
	{Header: "TAG", Value: func(item runtime.ImageListEntry) string { return item.Tag }},
	{Header: "IMAGE ID", Value: func(item runtime.ImageListEntry) string { return item.ID }},
	{Header: "CREATED", Value: func(item runtime.ImageListEntry) string { return item.Created }},
	{Header: "SIZE", Value: func(item runtime.ImageListEntry) string { return item.Size }},
}

var imageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List OCI images",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		flags := profile.DefaultFlags()
		flags = applyFlagOverrides(flags, cmd)
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))

		bin, err := runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}

		images, err := runtime.ListImages(bin)
		if err != nil {
			output.Printf("Error listing images: %v", err)
			os.Exit(1)
		}
		response := listResponse[runtime.ImageListEntry]{Kind: "image_list", Items: images}
		if err := renderOutput(cmd, response, func(writer io.Writer) error {
			return writeItemTable(writer, images, imageListTableFields, "No OCI images found.")
		}); err != nil {
			output.Printf("Error rendering images: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	addOutputFlag(imageListCmd)
	imageCmd.AddCommand(imageListCmd)
}
