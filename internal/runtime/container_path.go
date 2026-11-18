package runtime

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func resolveContainerRuntimePath(candidate string, lookPath func(string) (string, error), commandOutput func(string, ...string) ([]byte, error)) (string, error) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed != "" && trimmed != ContainerRuntimeBinary {
		path, err := lookPath(trimmed)
		if err != nil {
			return "", missingContainerRuntimeError(trimmed, err)
		}
		return path, nil
	}

	if path, err := resolveHomebrewContainerRuntimePath(lookPath, commandOutput); err == nil {
		return path, nil
	}

	path, err := lookPath(ContainerRuntimeBinary)
	if err != nil {
		return "", missingContainerRuntimeError(ContainerRuntimeBinary, err)
	}
	return path, nil
}

func resolveHomebrewContainerRuntimePath(lookPath func(string) (string, error), commandOutput func(string, ...string) ([]byte, error)) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", exec.ErrNotFound
	}
	brewPath, err := lookPath(ContainerBrewBinary)
	if err != nil {
		return "", err
	}
	out, err := commandOutput(brewPath, "--prefix", ContainerBrewFormula)
	if err != nil {
		return "", err
	}
	lines := SanitizeStructuredCommandLines(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("brew --prefix %s returned no output", ContainerBrewFormula)
	}
	prefix := strings.TrimSpace(lines[0])
	if prefix == "" {
		return "", fmt.Errorf("brew --prefix %s returned an empty prefix", ContainerBrewFormula)
	}
	candidate := filepath.Join(prefix, "bin", ContainerRuntimeBinary)
	path, err := lookPath(candidate)
	if err != nil {
		return "", err
	}
	return path, nil
}
