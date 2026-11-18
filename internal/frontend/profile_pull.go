package frontend

import (
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/logging"
	"nedgentic/internal/output"
	"nedgentic/internal/profile"
)

var profilePullCmd = &cobra.Command{
	Use:   "pull [profile]",
	Short: "Pull OCI images referenced by profiles",
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 && cmd.Flags().Changed("profile-path") {
			output.Print("Error: --profile-path requires a specific profile name")
			os.Exit(1)
		}

		if len(args) == 1 {
			name, err := normalizeAgentName("profile name", args[0])
			if err != nil {
				output.Printf("Error pulling profiles: %v", err)
				os.Exit(1)
			}
			if err := pullProfileImage(name, cmd); err != nil {
				output.Printf("Error pulling profile %s: %v", name, err)
				os.Exit(1)
			}
			return
		}

		if err := ensureProfilesInstalled(false, false); err != nil {
			output.Printf("Error ensuring profiles: %v", err)
			os.Exit(1)
		}

		names, err := collectProfileNames()
		if err != nil {
			output.Printf("Error listing profiles: %v", err)
			os.Exit(1)
		}
		var failed bool
		for _, name := range names {
			if err := pullProfileImage(name, cmd); err != nil {
				output.Printf("Error pulling profile %s: %v", name, err)
				failed = true
			}
		}
		if failed {
			os.Exit(1)
		}
	},
}

func init() {
	profileCmd.AddCommand(profilePullCmd)
}

func pullProfileImage(profileName string, cmd *cobra.Command) error {
	profilePath, err := resolveProfilePath(profileName)
	if err != nil {
		return err
	}
	prof, err := runtime.LoadProfile(profilePath, true, profileName)
	if err != nil {
		return err
	}
	flags := runtime.EffectiveFlags(prof)
	flags = applyFlagOverrides(flags, cmd)
	logging.SetLevel(logging.ParseLevel(flags.LogLevel))
	bin, err := runtime.ResolveContainerBin(flags)
	if err != nil {
		return err
	}
	imageRef, err := runtime.ResolveRunImage(prof)
	if err != nil {
		return err
	}
	output.Printf("Pulling image %s for profile %s...", imageRef, profileName)
	if err := ensureImagePull(bin, imageRef, flags.Platform); err != nil {
		return err
	}
	output.Print("Image pulled successfully.")
	return nil
}

func collectProfileNames() ([]string, error) {
	home, err := profile.ResolveHome()
	if err != nil {
		return nil, err
	}
	names := map[string]struct{}{
		profile.DefaultProfileName: {},
	}
	addNames := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				name := strings.TrimSpace(entry.Name())
				if name != "" {
					names[name] = struct{}{}
				}
			}
		}
		return nil
	}
	if err := addNames(profile.PersonalProfilesDir(home)); err != nil {
		return nil, err
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	return ordered, nil
}
