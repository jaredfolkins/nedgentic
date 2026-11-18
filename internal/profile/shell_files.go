package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nedgentic/internal/templates"
)

const (
	ShellBashrcName      = "bashrc"
	ShellBashProfileName = "bash_profile"
	ShellZshrcName       = "zshrc"
	ShellZprofileName    = "zprofile"
)

type shellFileSet struct {
	rcName       string
	profileName  string
	rcAsset      string
	profileAsset string
	rcVM         string
	profileVM    string
}

func EnsureProfileShellFiles(profileDir, shell string, force bool) error {
	if strings.TrimSpace(profileDir) == "" {
		return fmt.Errorf("profile dir is required")
	}
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return err
	}
	set := shellFilesForShell(shell)
	rcPath := filepath.Join(profileDir, set.rcName)
	profilePath := filepath.Join(profileDir, set.profileName)
	if err := writeShellTemplate(rcPath, set.rcAsset, force); err != nil {
		return err
	}
	if err := writeShellTemplate(profilePath, set.profileAsset, force); err != nil {
		return err
	}
	return nil
}

func ShellFileMounts(profileDir, shell string) []FileMount {
	set := shellFilesForShell(shell)
	return []FileMount{
		{
			HostPath: filepath.Join(profileDir, set.rcName),
			VMPath:   set.rcVM,
			Mode:     "copy",
		},
		{
			HostPath: filepath.Join(profileDir, set.profileName),
			VMPath:   set.profileVM,
			Mode:     "copy",
		},
	}
}

func ShellFileMountsForHome(profileDir, shell, home string) []FileMount {
	mounts := ShellFileMounts(profileDir, shell)
	for i := range mounts {
		mounts[i].HostPath = maybeTildePath(mounts[i].HostPath, home)
	}
	return mounts
}

func writeShellTemplate(path, asset string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	content, err := templates.Assets.ReadFile(asset)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", asset, err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		return err
	}
	return nil
}

func maybeTildePath(path, home string) string {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(home) == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.Join("~", rel)
}

func shellFilesForShell(shell string) shellFileSet {
	normalized := strings.ToLower(strings.TrimSpace(shell))
	if strings.Contains(normalized, "zsh") {
		return shellFileSet{
			rcName:       ShellZshrcName,
			profileName:  ShellZprofileName,
			rcAsset:      "shellconfig/zshrc",
			profileAsset: "shellconfig/zprofile",
			rcVM:         ".zshrc",
			profileVM:    ".zprofile",
		}
	}
	return shellFileSet{
		rcName:       ShellBashrcName,
		profileName:  ShellBashProfileName,
		rcAsset:      "shellconfig/bashrc",
		profileAsset: "shellconfig/bash_profile",
		rcVM:         ".bashrc",
		profileVM:    ".bash_profile",
	}
}
