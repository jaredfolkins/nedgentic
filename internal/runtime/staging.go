package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func ListStagingDirs() ([]string, string, error) {
	home, err := profile.ResolveHome()
	if err != nil {
		return nil, "", err
	}
	base := profile.StagingDir(home)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, base, nil
		}
		return nil, base, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			name := strings.TrimSpace(entry.Name())
			if name != "" {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names, base, nil
}

func CleanStagingAll() error {
	home, err := profile.ResolveHome()
	if err != nil {
		return err
	}
	base := profile.StagingDir(home)
	if strings.TrimSpace(base) == "" {
		return fmt.Errorf("staging base directory is required")
	}
	return os.RemoveAll(base)
}
