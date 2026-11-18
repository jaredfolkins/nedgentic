package frontend

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var (
	exportResolveContainerBin = runtime.ResolveContainerBin
	exportContainer           = runtime.ContainerExport
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export [vm]",
	Short: "Export a stopped VM as a tar archive",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		containerID, err := normalizeAgentName("vm name", args[0])
		if err != nil {
			output.Printf("Export failed: %v", err)
			os.Exit(1)
		}
		exportOutput, err = resolveExportOutputPath(cmd, exportOutput)
		if err != nil {
			output.Printf("Export failed: %v", err)
			os.Exit(1)
		}
		bin, err := resolveExportContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		if err := runWithSpinner("Export: "+containerID, func() error {
			return exportContainer(bin, containerID, exportOutput)
		}); err != nil {
			output.Printf("Export failed: %v", err)
			os.Exit(1)
		}
	},
}

func resolveExportOutputPath(cmd *cobra.Command, flagValue string) (string, error) {
	if cmd == nil || !cmd.Flags().Changed("output") {
		return "", nil
	}
	if strings.TrimSpace(flagValue) == "" {
		return "", fmt.Errorf("output path is required")
	}
	return normalizeAgentPath("output path", flagValue)
}

func resolveExportContainerBin(cmd *cobra.Command) (string, error) {
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	bin, err := exportResolveContainerBin(flags)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bin) == "" {
		return "", fmt.Errorf("container runtime is required for export commands")
	}
	return bin, nil
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Pathname for the exported tar archive (default: stdout)")
	rootCmd.AddCommand(exportCmd)
}
