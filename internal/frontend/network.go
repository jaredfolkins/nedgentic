package frontend

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var (
	networkResolveContainerBin = runtime.ResolveContainerBin
	networkCommandOutput       = func(name string, args ...string) ([]byte, error) {
		cmd := exec.Command(name, args...)
		cmd.Env = os.Environ()
		return cmd.CombinedOutput()
	}
)

var networkCreateInternal bool

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage container networks",
}

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List container networks",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		bin, err := resolveNetworkContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		out, err := runNetworkCommand(bin, "network", "list")
		if err != nil {
			output.Printf("Network list failed: %v", err)
			os.Exit(1)
		}
		printCommandOutputLines(out)
	},
}

var networkCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a container network",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, err := normalizeAgentName("network name", args[0])
		if err != nil {
			output.Printf("Network create failed: %v", err)
			os.Exit(1)
		}
		bin, err := resolveNetworkContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		var out []byte
		if err := runWithSpinner("Create network: "+name, func() error {
			var runErr error
			out, runErr = runNetworkCommand(bin, networkCreateArgs(name, networkCreateInternal)...)
			return runErr
		}); err != nil {
			output.Printf("Network create failed: %v", err)
			os.Exit(1)
		}
		printCommandOutputLines(out)
	},
}

var networkRemoveCmd = &cobra.Command{
	Use:     "remove [name]...",
	Aliases: []string{"rm"},
	Short:   "Remove one or more container networks",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bin, err := resolveNetworkContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		for _, raw := range args {
			name, err := normalizeAgentName("network name", raw)
			if err != nil {
				output.Printf("Network remove failed: %v", err)
				os.Exit(1)
			}
			var out []byte
			if err := runWithSpinner("Remove network: "+name, func() error {
				var runErr error
				out, runErr = runNetworkCommand(bin, networkRemoveArgs(name)...)
				return runErr
			}); err != nil {
				output.Printf("Network remove failed (%s): %v", name, err)
				os.Exit(1)
			}
			printCommandOutputLines(out)
		}
	},
}

func resolveNetworkContainerBin(cmd *cobra.Command) (string, error) {
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	bin, err := networkResolveContainerBin(flags)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bin) == "" {
		return "", fmt.Errorf("container runtime is required for network commands")
	}
	return bin, nil
}

func networkCreateArgs(name string, internal bool) []string {
	args := []string{"network", "create"}
	if internal {
		args = append(args, "--internal")
	}
	args = append(args, name)
	return args
}

func networkRemoveArgs(name string) []string {
	return []string{"network", "delete", name}
}

func runNetworkCommand(bin string, args ...string) ([]byte, error) {
	out, err := networkCommandOutput(bin, args...)
	if err != nil {
		return out, formatCommandOutputError(err, out)
	}
	return out, nil
}

func printCommandOutputLines(out []byte) {
	for _, line := range sanitizeCommandOutputLines(out) {
		output.Print(line)
	}
}

func init() {
	networkCreateCmd.Flags().BoolVar(&networkCreateInternal, "internal", false, "Create a host-only internal network")
	networkCmd.AddCommand(networkListCmd)
	networkCmd.AddCommand(networkCreateCmd)
	networkCmd.AddCommand(networkRemoveCmd)
	rootCmd.AddCommand(networkCmd)
}
