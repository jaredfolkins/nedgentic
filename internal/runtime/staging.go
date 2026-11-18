package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nedgentic/internal/profile"
)

func EnsureStagingDir(vmName string, clean bool) (string, error) {
	home, err := profile.ResolveHome()
	if err != nil {
		return "", err
	}
	base := profile.StagingDir(home)
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(base, 0700); err != nil {
		return "", err
	}
	path, err := profile.StagingVMDir(home, vmName)
	if err != nil {
		return "", err
	}
	if clean {
		if err := os.RemoveAll(path); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}

// RemoveStagingDir deletes a VM's host staging directory, if any.
func RemoveStagingDir(vmName string) error {
	home, err := profile.ResolveHome()
	if err != nil {
		return err
	}
	path, err := profile.StagingVMDir(home, vmName)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func EnsureRunnerDir(stagingRoot string) (string, error) {
	if strings.TrimSpace(stagingRoot) == "" {
		return "", fmt.Errorf("staging root is required")
	}
	path := filepath.Join(stagingRoot, "runner")
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}
