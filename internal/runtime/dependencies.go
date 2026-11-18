package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"nedgentic/internal/profile"
)

const dependencyContainer = "container"

type DependencyState struct {
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	// Outdated marks an installed dependency below Ned's minimum version; it is
	// upgraded through the same install path as a missing dependency.
	Outdated  bool      `json:"outdated,omitempty"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type DependenciesState struct {
	UpdatedAt    time.Time         `json:"updated_at"`
	Dependencies []DependencyState `json:"dependencies"`
}

var (
	depsLookPath      = exec.LookPath
	depsCommandOutput = runtimeCommandOutput
	depsMkdirAll      = os.MkdirAll
	depsWriteFile     = os.WriteFile
)

func CheckDependencies(flags profile.Flags) (DependenciesState, error) {
	now := time.Now().UTC()
	return DependenciesState{
		UpdatedAt:    now,
		Dependencies: []DependencyState{checkContainerDependency(flags, now)},
	}, nil
}

func MissingRequiredDependencies(state DependenciesState) []DependencyState {
	missing := make([]DependencyState, 0, len(state.Dependencies))
	for _, dep := range state.Dependencies {
		if dep.Required && (!dep.Installed || dep.Outdated) {
			missing = append(missing, dep)
		}
	}
	return missing
}

func InstallDependency(name string, out io.Writer) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case dependencyContainer:
		return InstallContainerRuntime(out, RuntimeActionOptions{})
	default:
		return fmt.Errorf("unknown dependency %q", name)
	}
}

func WriteDependenciesState(home string, state DependenciesState) error {
	if strings.TrimSpace(home) == "" {
		return fmt.Errorf("home directory is required")
	}
	if err := depsMkdirAll(profile.BaseDir(home), 0700); err != nil {
		return fmt.Errorf("failed to create dependency state dir: %v", err)
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize dependency state: %v", err)
	}
	path := profile.DependenciesPath(home)
	if err := depsWriteFile(path, append(payload, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write dependency state: %v", err)
	}
	return nil
}

func checkContainerDependency(flags profile.Flags, now time.Time) DependencyState {
	state := DependencyState{
		Name:      dependencyContainer,
		Required:  true,
		CheckedAt: now,
	}
	path, err := resolveContainerRuntimePath(flags.ContainerBin, depsLookPath, depsCommandOutput)
	if err != nil {
		state.Error = err.Error()
		return state
	}
	state.Installed = true
	state.Path = path
	version, err := containerVersion(path)
	if err != nil {
		state.Error = err.Error()
		return state
	}
	state.Version = version
	if err := checkContainerVersion(version); err != nil {
		state.Outdated = true
		state.Error = err.Error()
	}
	return state
}
