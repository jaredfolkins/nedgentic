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
	runtimeDNSResolveContainerBin = runtime.ResolveContainerBin
	runtimeDNSCommandOutput       = func(name string, args ...string) ([]byte, error) {
		cmd := exec.Command(name, args...)
		cmd.Env = os.Environ()
		return cmd.CombinedOutput()
	}
)

var (
	runtimeDNSCreateLocalhost []string
	runtimeDNSListFormat      string
	runtimeDNSListQuiet       bool
)

var runtimeDNSCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage runtime DNS domains",
}

var runtimeDNSListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List runtime DNS domains",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		bin, err := resolveRuntimeDNSContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		out, err := runRuntimeDNSCommand(bin, runtimeDNSListArgs(runtimeDNSListFormat, runtimeDNSListQuiet)...)
		if err != nil {
			output.Printf("Runtime DNS list failed: %v", err)
			os.Exit(1)
		}
		if err := writeCommandOutputText(commandStdout(cmd), out); err != nil {
			output.Printf("Runtime DNS list failed: %v", err)
			os.Exit(1)
		}
	},
}

var runtimeDNSCreateCmd = &cobra.Command{
	Use:   "create [domain]",
	Short: "Create a runtime DNS domain",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		domain, err := normalizeAgentName("runtime DNS domain", args[0])
		if err != nil {
			output.Printf("Runtime DNS create failed: %v", err)
			os.Exit(1)
		}
		localhost := cleanStringList(runtimeDNSCreateLocalhost)
		bin, err := resolveRuntimeDNSContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		var out []byte
		if err := runWithSpinner("Create runtime DNS domain: "+domain, func() error {
			var runErr error
			out, runErr = runRuntimeDNSCommand(bin, runtimeDNSCreateArgs(domain, localhost)...)
			return runErr
		}); err != nil {
			output.Printf("Runtime DNS create failed: %v", err)
			os.Exit(1)
		}
		printCommandOutputLines(out)
	},
}

var runtimeDNSDeleteCmd = &cobra.Command{
	Use:     "delete [domain]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a runtime DNS domain",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		domain, err := normalizeAgentName("runtime DNS domain", args[0])
		if err != nil {
			output.Printf("Runtime DNS delete failed: %v", err)
			os.Exit(1)
		}
		bin, err := resolveRuntimeDNSContainerBin(cmd)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		var out []byte
		if err := runWithSpinner("Delete runtime DNS domain: "+domain, func() error {
			var runErr error
			out, runErr = runRuntimeDNSCommand(bin, runtimeDNSDeleteArgs(domain)...)
			return runErr
		}); err != nil {
			output.Printf("Runtime DNS delete failed: %v", err)
			os.Exit(1)
		}
		printCommandOutputLines(out)
	},
}

func resolveRuntimeDNSContainerBin(cmd *cobra.Command) (string, error) {
	flags := applyFlagOverrides(profile.DefaultFlags(), cmd)
	bin, err := runtimeDNSResolveContainerBin(flags)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bin) == "" {
		return "", fmt.Errorf("container runtime is required for runtime dns commands")
	}
	return bin, nil
}

func runtimeDNSListArgs(format string, quiet bool) []string {
	args := []string{"system", "dns", "list"}
	if trimmed := strings.TrimSpace(format); trimmed != "" {
		args = append(args, "--format", trimmed)
	}
	if quiet {
		args = append(args, "--quiet")
	}
	return args
}

func runtimeDNSCreateArgs(domain string, localhost []string) []string {
	args := []string{"system", "dns", "create"}
	for _, mapping := range cleanStringList(localhost) {
		args = append(args, "--localhost", mapping)
	}
	args = append(args, domain)
	return args
}

func runtimeDNSDeleteArgs(domain string) []string {
	return []string{"system", "dns", "delete", domain}
}

func runRuntimeDNSCommand(bin string, args ...string) ([]byte, error) {
	out, err := runtimeDNSCommandOutput(bin, args...)
	if err != nil {
		return out, formatCommandOutputError(err, out)
	}
	return out, nil
}

func init() {
	runtimeDNSListCmd.Flags().StringVar(&runtimeDNSListFormat, "format", "", "Output format (table or json)")
	runtimeDNSListCmd.Flags().BoolVarP(&runtimeDNSListQuiet, "quiet", "q", false, "Only print domain names")
	runtimeDNSCreateCmd.Flags().StringArrayVar(&runtimeDNSCreateLocalhost, "localhost", nil, "Add host-service DNS mapping (repeatable)")
	runtimeDNSCmd.AddCommand(runtimeDNSListCmd)
	runtimeDNSCmd.AddCommand(runtimeDNSCreateCmd)
	runtimeDNSCmd.AddCommand(runtimeDNSDeleteCmd)
	runtimeCmd.AddCommand(runtimeDNSCmd)
}
