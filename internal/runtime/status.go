package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type RuntimeStatus struct {
	ContainerPath         string
	ContainerVersion      string
	ContainerVersionError string
	BrewPath              string
	BrewContainerVersion  string
	BrewContainerError    string
}

var (
	statusLookPath      = exec.LookPath
	statusCommandOutput = runtimeCommandOutput
)

func WriteRuntimeStatus(out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	status := collectRuntimeStatus()
	_, err := io.WriteString(out, formatRuntimeStatus(status))
	return err
}

func CollectRuntimeStatus() RuntimeStatus {
	return collectRuntimeStatus()
}

func collectRuntimeStatus() RuntimeStatus {
	status := RuntimeStatus{}

	if path, err := resolveContainerRuntimePath("", statusLookPath, statusCommandOutput); err == nil {
		status.ContainerPath = path
		if version, err := containerVersion(path); err == nil {
			status.ContainerVersion = version
		} else {
			status.ContainerVersionError = err.Error()
		}
	}

	if path, err := statusLookPath(ContainerBrewBinary); err == nil {
		status.BrewPath = path
		if version, err := brewContainerVersion(path); err == nil {
			status.BrewContainerVersion = version
		} else {
			status.BrewContainerError = err.Error()
		}
	}

	return status
}

func formatRuntimeStatus(status RuntimeStatus) string {
	var builder strings.Builder
	builder.WriteString("Runtime status:\n")

	if strings.TrimSpace(status.ContainerPath) == "" {
		builder.WriteString("  container: not found\n")
		builder.WriteString("  install: ned runtime install\n")
	} else {
		builder.WriteString("  container: installed\n")
		builder.WriteString(fmt.Sprintf("  container path: %s\n", status.ContainerPath))
		if strings.TrimSpace(status.ContainerVersion) != "" {
			builder.WriteString(fmt.Sprintf("  container version: %s\n", status.ContainerVersion))
		} else if strings.TrimSpace(status.ContainerVersionError) != "" {
			builder.WriteString(fmt.Sprintf("  container version: unavailable (%s)\n", status.ContainerVersionError))
		} else {
			builder.WriteString("  container version: unavailable\n")
		}
	}

	if strings.TrimSpace(status.BrewPath) == "" {
		builder.WriteString("  homebrew: not found\n")
	} else {
		builder.WriteString("  homebrew: installed\n")
		builder.WriteString(fmt.Sprintf("  homebrew path: %s\n", status.BrewPath))
		if strings.TrimSpace(status.BrewContainerVersion) != "" {
			builder.WriteString(fmt.Sprintf("  homebrew container: %s\n", status.BrewContainerVersion))
		} else if strings.TrimSpace(status.BrewContainerError) != "" {
			if strings.TrimSpace(status.ContainerPath) != "" {
				builder.WriteString("  homebrew container: <installed on system>\n")
			} else {
				builder.WriteString("  homebrew container: not found\n")
			}
		} else {
			builder.WriteString("  homebrew container: not found\n")
		}
	}

	return builder.String()
}

func containerVersion(path string) (string, error) {
	out, err := statusCommandOutput(path, "--version")
	if err != nil {
		return "", fmt.Errorf("version check failed: %v", err)
	}
	lines := SanitizeStructuredCommandLines(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("version output was empty")
	}
	return lines[0], nil
}

func brewContainerVersion(path string) (string, error) {
	out, err := statusCommandOutput(path, "list", "--versions", ContainerBrewFormula)
	if err != nil {
		return "", err
	}
	lines := SanitizeStructuredCommandLines(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("no version output")
	}
	return lines[0], nil
}

func runtimeCommandOutput(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}
