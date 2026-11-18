package frontend

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var (
	runtimeStatusFormat               string
	runtimeStatusWriteRuntimeStatus   = runtime.WriteRuntimeStatus
	runtimeStatusCollectRuntimeStatus = runtime.CollectRuntimeStatus
	runtimeStatusResolveContainerBin  = runtime.ResolveContainerBin
	runtimeStatusSystemStatus         = runtime.ContainerSystemStatus
)

var runtimeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show container runtime status",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if currentOutputMode() == output.ModeText && strings.TrimSpace(runtimeStatusFormat) != "" {
			bin, err := resolveRuntimeStatusContainerBin(cmd)
			if err != nil {
				printRuntimeResolveError(err)
				os.Exit(1)
			}
			out, err := runtimeStatusSystemStatus(bin, runtimeStatusFormat)
			if err != nil {
				output.Printf("Runtime status failed: %v", err)
				os.Exit(1)
			}
			printCommandOutputLines(out)
			return
		}
		status := runtimeStatusCollectRuntimeStatus()
		response := singleResponse[runtime.RuntimeStatus]{Kind: "runtime_status", Item: status}
		if err := renderOutput(cmd, response, func(writer io.Writer) error {
			var buf strings.Builder
			if err := runtimeStatusWriteRuntimeStatus(&buf); err != nil {
				return err
			}
			return writeCommandOutputText(writer, []byte(buf.String()))
		}); err != nil {
			output.Printf("Runtime status failed: %v", err)
			os.Exit(1)
		}
	},
}

func resolveRuntimeStatusContainerBin(cmd *cobra.Command) (string, error) {
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	bin, err := runtimeStatusResolveContainerBin(flags)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bin) == "" {
		return "", fmt.Errorf("container runtime is required for runtime status --format")
	}
	return bin, nil
}

func init() {
	runtimeStatusCmd.Flags().StringVar(&runtimeStatusFormat, "format", "", "Output format (table, json, or yaml via container system status)")
	addOutputFlag(runtimeStatusCmd)
	runtimeCmd.AddCommand(runtimeStatusCmd)
}
