package frontend

import (
	"os"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var execCmd = &cobra.Command{
	Use:   "exec [vm] [command]...",
	Short: "Execute a command in a running VM",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		command := args[1:]
		bin, err := runtime.ResolveContainerBin(profile.DefaultFlags())
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		prof, err := resolveProfileForExec(bin, name)
		if err != nil {
			output.Printf("Error resolving profile defaults: %v", err)
			os.Exit(1)
		}
		flags := profile.DefaultFlags()
		if prof != nil {
			flags = runtime.EffectiveFlags(prof)
		}
		bin, err = runtime.ResolveContainerBin(flags)
		if err != nil {
			printRuntimeResolveError(err)
			os.Exit(1)
		}
		logging.SetLevel(logging.ParseLevel(flags.LogLevel))
		env := runtime.ProxyEnvEntries(flags)
		argv, err := runtime.BuildExecArgs(runtime.ExecOptions{
			TTY:           runtime.WantsTTY(),
			Env:           env,
			Ulimits:       flags.Ulimits,
			ContainerName: name,
			Command:       command,
		})
		if err != nil {
			output.Printf("Error building exec command: %v", err)
			os.Exit(1)
		}
		if err := runtime.RunContainerExec(bin, argv); err != nil {
			output.Printf("Error executing command: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
}

func resolveProfileForExec(bin, name string) (*runtime.Profile, error) {
	entries, err := runtime.ListContainers(bin)
	if err != nil {
		return nil, err
	}
	entry, ok := findContainerEntry(entries, name)
	if !ok {
		return nil, nil
	}
	return profileForEntry(entry)
}

// profileForEntry loads the profile a Ned VM was started from, or nil when
// the VM carries no profile label.
func profileForEntry(entry runtime.ContainerListEntry) (*runtime.Profile, error) {
	profileName := entry.Configuration.Labels[runtime.LabelProfileKey]
	if profileName == "" {
		return nil, nil
	}
	return runtime.LoadProfile("", false, profileName)
}
