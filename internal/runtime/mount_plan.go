package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"nedgentic/internal/config"
	"nedgentic/internal/logging"
	"nedgentic/internal/profile"
)

func validateContainerPath(path string) error {
	if strings.Contains(path, ",") {
		return fmt.Errorf("paths cannot contain commas for container mounts: %s", path)
	}
	return nil
}

func resolvePathForHomeCheck(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func ensureUnderHome(path, home string) error {
	homeResolved, err := resolvePathForHomeCheck(home)
	if err != nil {
		return fmt.Errorf("host home is required: %w", err)
	}
	pathResolved, err := resolvePathForHomeCheck(path)
	if err != nil {
		return fmt.Errorf("host path resolution failed: %w", err)
	}
	rel, err := filepath.Rel(homeResolved, pathResolved)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path must be under host home: %s", path)
	}
	return nil
}

func ensureUnderRoot(root, dest string) error {
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return err
	}
	if rel == "." {
		return fmt.Errorf("staging destination resolves to root")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("staging destination escapes root: %q", dest)
	}
	return nil
}

func ensureNoSymlink(root, dest string) error {
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	current := root
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("staging path contains symlink: %s", current)
		}
	}
	return nil
}

func redactPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "<redacted>"
	}
	base := filepath.Base(trimmed)
	if base == "." || base == string(os.PathSeparator) || base == "" {
		return "<redacted>"
	}
	return base
}

func normalizeMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func normalizeAccess(access string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(access))
	if value == "" {
		return false, nil
	}
	switch value {
	case config.AccessReadOnly:
		return true, nil
	case config.AccessReadWrite:
		return false, nil
	default:
		return false, fmt.Errorf("invalid access %q", access)
	}
}

func secretFileMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return 0600
	}
	if perm&0200 != 0 {
		return 0600
	}
	return 0400
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if err := out.Chmod(mode); err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	return err
}

func copyDir(root, src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			logging.Warnf("Skipping symlink %s", path)
			return nil
		}
		if entry.IsDir() {
			if err := ensureNoSymlink(root, target); err != nil {
				return err
			}
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			if err := os.Chmod(target, 0700); err != nil {
				return err
			}
			return nil
		}
		if err := ensureNoSymlink(root, target); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := os.Chmod(filepath.Dir(target), 0700); err != nil {
			return err
		}
		mode := secretFileMode(info.Mode())
		if err := copyFile(path, target, mode); err != nil {
			return err
		}
		return nil
	})
}

func MapHostPathToGuest(hostPath, hostHome, username string) (string, bool, error) {
	hostHome = strings.TrimSpace(hostHome)
	if hostHome == "" {
		return "", false, nil
	}
	rel, err := filepath.Rel(hostHome, hostPath)
	if err != nil {
		return "", false, err
	}
	if rel == "." {
		return GuestHomeForUser(username), true, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false, nil
	}
	return filepath.Join(GuestHomeForUser(username), rel), true, nil
}

func ResolveTargetPath(hostPath, vmPath, hostHome, username string) (string, error) {
	vmPath = strings.TrimSpace(vmPath)
	if vmPath != "" && containsParentSegment(vmPath) {
		return "", fmt.Errorf("vm path cannot contain '..': %s", vmPath)
	}
	if vmPath == "" {
		mapped, ok, err := MapHostPathToGuest(hostPath, hostHome, username)
		if err != nil {
			return "", err
		}
		if ok {
			return filepath.Clean(mapped), nil
		}
		return hostPath, nil
	}
	if strings.HasPrefix(vmPath, "~/") {
		return filepath.Clean(filepath.Join(GuestHomeForUser(username), vmPath[2:])), nil
	}
	if filepath.IsAbs(vmPath) {
		return filepath.Clean(vmPath), nil
	}
	return filepath.Clean(filepath.Join(GuestHomeForUser(username), vmPath)), nil
}

func containsParentSegment(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}

func workspaceMountPath(hostPath string) string {
	cleaned := filepath.Clean(hostPath)
	trimmed := strings.TrimPrefix(cleaned, string(os.PathSeparator))
	if trimmed == "" {
		return WorkspaceMountDir
	}
	return filepath.Join(WorkspaceMountDir, trimmed)
}

func PlanMounts(files []profile.FileMount, dirs []profile.DirectoryMount, username, hostHome, manualStagingRoot string) (*MountPlan, func(), error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return nil, nil, err
	}
	hostHomeResolved, err := resolvePathForHomeCheck(hostHome)
	if err != nil {
		return nil, nil, fmt.Errorf("host home is required: %w", err)
	}
	if err := ensureUnderHome(cwd, hostHomeResolved); err != nil {
		return nil, nil, fmt.Errorf("workspace must be under host home: %w", err)
	}
	workspaceTarget := cwd
	mapped, ok, err := MapHostPathToGuest(cwd, hostHomeResolved, username)
	if err != nil {
		return nil, nil, err
	}
	if ok {
		workspaceTarget = filepath.Clean(mapped)
	}
	if err := validateContainerPath(workspaceTarget); err != nil {
		return nil, nil, err
	}

	workspaceMount := workspaceMountPath(cwd)
	if err := validateContainerPath(workspaceMount); err != nil {
		return nil, nil, err
	}

	plan := &MountPlan{Mounts: []ContainerMount{{Type: MountTypeBind, Source: cwd, Target: workspaceMount, ReadOnly: false}},
		WorkspacePath:  workspaceTarget,
		WorkspaceMount: workspaceMount,
	}
	seenTargets := map[string]struct{}{workspaceMount: {}}

	stagingRoot := manualStagingRoot
	cleanup := func() {
		if stagingRoot != "" && manualStagingRoot == "" {
			_ = os.RemoveAll(stagingRoot)
		}
	}

	for _, m := range files {
		rawHostPath := strings.TrimSpace(m.HostPath)
		hostPath, err := ExpandHome(rawHostPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("file import %q: %w", rawHostPath, err)
		}
		hostPath, err = filepath.Abs(hostPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("file import %q: %w", rawHostPath, err)
		}
		info, err := os.Stat(hostPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("file import %q: %w", rawHostPath, err)
		}
		targetPath, err := ResolveTargetPath(hostPath, m.VMPath, hostHomeResolved, username)
		if err != nil {
			return nil, cleanup, err
		}
		if !filepath.IsAbs(targetPath) {
			return nil, cleanup, fmt.Errorf("vm path must be absolute: %s", targetPath)
		}
		if err := validateContainerPath(targetPath); err != nil {
			return nil, cleanup, err
		}

		mode := normalizeMode(m.Mode)
		if mode == "" {
			mode = config.MountModeCopy
		}
		if mode != config.MountModeCopy {
			return nil, cleanup, fmt.Errorf("file entries only support mode %q: %s", config.MountModeCopy, hostPath)
		}

		if stagingRoot == "" {
			stagingRoot = filepath.Join(hostHomeResolved, profile.BaseDirName, config.StagingDirName, fmt.Sprintf("%d", os.Getpid()))
			if err := os.MkdirAll(stagingRoot, 0700); err != nil {
				return nil, cleanup, err
			}
			if err := os.Chmod(stagingRoot, 0700); err != nil {
				return nil, cleanup, err
			}
		}

		rel := strings.TrimPrefix(targetPath, string(os.PathSeparator))
		stagingDest := filepath.Join(stagingRoot, rel)
		if err := ensureUnderRoot(stagingRoot, stagingDest); err != nil {
			return nil, cleanup, err
		}
		if err := ensureNoSymlink(stagingRoot, stagingDest); err != nil {
			return nil, cleanup, err
		}
		if err := os.MkdirAll(filepath.Dir(stagingDest), 0700); err != nil {
			return nil, cleanup, fmt.Errorf("stage file import %q: create staging dir: %w", rawHostPath, err)
		}
		if err := os.Chmod(filepath.Dir(stagingDest), 0700); err != nil {
			return nil, cleanup, fmt.Errorf("stage file import %q: secure staging dir: %w", rawHostPath, err)
		}
		fileMode := secretFileMode(info.Mode())
		if err := copyFile(hostPath, stagingDest, fileMode); err != nil {
			return nil, cleanup, fmt.Errorf("stage file import %q: %w", rawHostPath, err)
		}
		plan.CopySpecs = append(plan.CopySpecs, FileCopySpec{SourceRel: rel, DestPath: targetPath, Mode: fileMode})
	}

	for _, d := range dirs {
		rawHostPath := strings.TrimSpace(d.HostPath)
		hostPath, err := ExpandHome(rawHostPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("directory import %q: %w", rawHostPath, err)
		}
		hostPath, err = filepath.Abs(hostPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("directory import %q: %w", rawHostPath, err)
		}
		info, err := os.Stat(hostPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("directory import %q: %w", rawHostPath, err)
		}
		if err := ensureUnderHome(hostPath, hostHomeResolved); err != nil {
			return nil, cleanup, fmt.Errorf("directory mount must be under host home: %w", err)
		}
		if !info.IsDir() {
			return nil, cleanup, fmt.Errorf("directory entries must be directories: %s", hostPath)
		}

		targetPath, err := ResolveTargetPath(hostPath, d.VMPath, hostHomeResolved, username)
		if err != nil {
			return nil, cleanup, err
		}
		if !filepath.IsAbs(targetPath) {
			return nil, cleanup, fmt.Errorf("vm path must be absolute: %s", targetPath)
		}
		if err := validateContainerPath(targetPath); err != nil {
			return nil, cleanup, err
		}

		mode := normalizeMode(d.Mode)
		if mode == "" {
			mode = config.MountModeMount
		}
		switch mode {
		case config.MountModeMount:
			if err := validateContainerPath(hostPath); err != nil {
				return nil, cleanup, err
			}
			readOnlyValue, err := normalizeAccess(d.Access)
			if err != nil {
				return nil, cleanup, err
			}
			if _, exists := seenTargets[targetPath]; exists {
				continue
			}
			seenTargets[targetPath] = struct{}{}
			plan.Mounts = append(plan.Mounts, ContainerMount{
				Source:   hostPath,
				Target:   targetPath,
				ReadOnly: readOnlyValue,
			})
		case config.MountModeCopy:
			if strings.TrimSpace(d.Access) != "" {
				return nil, cleanup, fmt.Errorf("directory entry cannot set access when mode is %q: %s", config.MountModeCopy, hostPath)
			}
			if stagingRoot == "" {
				stagingRoot = filepath.Join(hostHomeResolved, profile.BaseDirName, config.StagingDirName, fmt.Sprintf("%d", os.Getpid()))
				if err := os.MkdirAll(stagingRoot, 0700); err != nil {
					return nil, cleanup, err
				}
				if err := os.Chmod(stagingRoot, 0700); err != nil {
					return nil, cleanup, err
				}
			}
			rel := strings.TrimPrefix(targetPath, string(os.PathSeparator))
			stagingDest := filepath.Join(stagingRoot, rel)
			if err := ensureUnderRoot(stagingRoot, stagingDest); err != nil {
				return nil, cleanup, err
			}
			if err := ensureNoSymlink(stagingRoot, stagingDest); err != nil {
				return nil, cleanup, err
			}
			if err := copyDir(stagingRoot, hostPath, stagingDest); err != nil {
				return nil, cleanup, fmt.Errorf("stage directory import %q: %w", rawHostPath, err)
			}
			plan.CopySpecs = append(plan.CopySpecs, FileCopySpec{SourceRel: rel, DestPath: targetPath, Mode: info.Mode(), IsDir: true})
		default:
			return nil, cleanup, fmt.Errorf("directory entries must use mode %q or %q: %s", config.MountModeMount, config.MountModeCopy, hostPath)
		}
	}

	if stagingRoot != "" {
		if err := ensureUnderHome(stagingRoot, hostHomeResolved); err != nil {
			return nil, cleanup, fmt.Errorf("staging root must be under host home: %w", err)
		}
		if err := validateContainerPath(SecretsMountPath); err != nil {
			return nil, cleanup, err
		}
		plan.StagingRoot = stagingRoot
		plan.Mounts = append(plan.Mounts, ContainerMount{
			Source:   stagingRoot,
			Target:   SecretsMountPath,
			ReadOnly: true,
		})
	}

	return plan, cleanup, nil
}
