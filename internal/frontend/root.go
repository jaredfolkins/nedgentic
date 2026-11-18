package frontend

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var rootCmd = &cobra.Command{
	Use:   "ned",
	Short: "Ned: sandboxed agent VMs for macOS",
	Long:  "Ned starts a sandboxed Linux VM (via Apple's container runtime) with the current folder mounted read-write and attaches a shell. Run `ned gentic <profile> [vm]` to start.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if shouldSkipDependencyCheck(cmd) {
			return nil
		}
		if err := ensureDependencies(cmd, profile.DefaultFlags()); err != nil {
			output.Printf("Dependency check failed: %v", err)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
}

func Execute(buildTime string) {
	configureOutput()
	if buildTime != "dev" {
		output.Printf("Build date: %s", buildTime)
	}
	logging.SetLevel(logging.ParseLevel(profile.DefaultFlags().LogLevel))
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func shouldSkipDependencyCheck(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}
	return cmd.Parent() == nil || cmd.Name() == "help"
}

func ensureDependencies(cmd *cobra.Command, flags profile.Flags) error {
	if err := runtime.CheckHostPlatform(); err != nil {
		return err
	}
	hostUser, err := runtime.ResolveHostUser()
	if err != nil {
		return err
	}
	state, err := runtime.CheckDependencies(flags)
	if err != nil {
		return err
	}
	missing := runtime.MissingRequiredDependencies(state)
	if len(missing) > 0 {
		for _, dep := range missing {
			label := dependencyInstallLabel(dep)
			if err := runWithSpinner(label, func() error {
				return runtime.InstallDependency(dep.Name, cmd.OutOrStdout())
			}); err != nil {
				finalState, _ := runtime.CheckDependencies(flags)
				_ = runtime.WriteDependenciesState(hostUser.Home, finalState)
				return err
			}
		}
		state, err = runtime.CheckDependencies(flags)
		if err != nil {
			return err
		}
	}
	if err := ensureContainerRuntimeService(state); err != nil {
		return err
	}
	if err := runtime.WriteDependenciesState(hostUser.Home, state); err != nil {
		return err
	}
	if missing := runtime.MissingRequiredDependencies(state); len(missing) > 0 {
		return fmt.Errorf("missing required dependencies: %s (see %s)", formatDependencyList(missing), profile.DependenciesPath(hostUser.Home))
	}
	return nil
}

func ensureContainerRuntimeService(state runtime.DependenciesState) error {
	bin, ok := installedDependencyPath(state, "container")
	if !ok {
		return nil
	}
	running, statusErr := runtime.ContainerRuntimeServiceRunning(bin)
	if statusErr == nil && running {
		return nil
	}
	if err := runWithSpinner("Start container runtime:", func() error {
		return runtime.StartContainerRuntimeService(bin)
	}); err != nil {
		if statusErr != nil {
			return fmt.Errorf("container service status check failed: %v; %w", statusErr, err)
		}
		return err
	}
	return nil
}

func installedDependencyPath(state runtime.DependenciesState, name string) (string, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return "", false
	}
	for _, dep := range state.Dependencies {
		if !dep.Installed {
			continue
		}
		if strings.ToLower(strings.TrimSpace(dep.Name)) != target {
			continue
		}
		path := strings.TrimSpace(dep.Path)
		if path == "" {
			return "", false
		}
		return path, true
	}
	return "", false
}

func dependencyInstallLabel(dep runtime.DependencyState) string {
	verb := "Install"
	if dep.Outdated {
		verb = "Upgrade"
	}
	name := strings.TrimSpace(dep.Name)
	switch strings.ToLower(name) {
	case "container":
		return verb + " container runtime:"
	default:
		return fmt.Sprintf("Install %s:", strings.TrimSpace(name))
	}
}

func formatDependencyList(deps []runtime.DependencyState) string {
	names := make([]string, 0, len(deps))
	for _, dep := range deps {
		name := strings.TrimSpace(dep.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "unknown"
	}
	return strings.Join(names, ", ")
}
