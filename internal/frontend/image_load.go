package frontend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var imageLoadCmd = &cobra.Command{
	Use:   "load <archive>",
	Short: "Load an OCI image archive",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		archivePath, err := normalizeImageLoadArchivePath(args[0])
		if err != nil {
			output.Printf("Error loading image: %v", err)
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
		if err := runWithSpinner("Load image:", func() error {
			return runtime.ContainerImageLoad(bin, archivePath)
		}); err != nil {
			output.Printf("Error loading image: %v", err)
			os.Exit(1)
		}
		output.Print("Image loaded successfully.")
	},
}

func init() {
	imageCmd.AddCommand(imageLoadCmd)
}

func normalizeImageLoadArchivePath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", fmt.Errorf("archive path is required")
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home
	} else if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		abs, err := filepath.Abs(clean)
		if err != nil {
			return "", err
		}
		clean = abs
	}
	info, err := os.Stat(clean)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("archive path is a directory: %s", clean)
	}
	return clean, nil
}
