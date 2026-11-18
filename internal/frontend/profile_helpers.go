package frontend

import (
	"strings"

	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/profile"
)

func resolveProfilePath(profileName string) (string, error) {
	normalizedName, err := normalizeAgentName("profile name", profileName)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(flagProfilePath) != "" {
		return flagProfilePath, nil
	}
	home, err := profile.ResolveHome()
	if err != nil {
		return "", err
	}
	return profile.ProfileConfigPath(home, normalizedName), nil
}

func ensureProfilesInstalled(overwrite, verbose bool) error {
	home, err := profile.ResolveHome()
	if err != nil {
		return err
	}
	defaultPath := profile.DefaultProfilePath(home)
	if overwrite {
		var err error
		if verbose {
			err = runWithSpinner("Overwrite default profile: "+profile.DefaultProfileName, func() error {
				return runtime.WriteProfileTemplate(defaultPath, "", true)
			})
		} else {
			err = runtime.WriteProfileTemplate(defaultPath, "", true)
		}
		if err != nil {
			return err
		}
	} else if _, err := profile.EnsureDefaultProfile(home); err != nil {
		return err
	}
	return nil
}
