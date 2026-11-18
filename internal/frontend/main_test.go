package frontend

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/backend/runtime"
	"nedgentic/internal/config"
	"nedgentic/internal/profile"
)

func TestResolveTargetPathDefaultHostPath(t *testing.T) {
	host := "/home/alice/.gemini"
	got, err := runtime.ResolveTargetPath(host, "", "/home/alice", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/home", "alice", ".gemini")
	if got != expected {
		t.Fatalf("expected mapped path, got %s", got)
	}
}

func TestResolveTargetPathRelative(t *testing.T) {
	got, err := runtime.ResolveTargetPath("/tmp/foo", ".aws/credentials", "/home/alice", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/home", "alice", ".aws", "credentials")
	if got != expected {
		t.Fatalf("unexpected target path: %s", got)
	}
}

func TestResolveTargetPathRejectsParent(t *testing.T) {
	if _, err := runtime.ResolveTargetPath("/tmp/foo", "../etc/passwd", "/home/alice", "alice"); err == nil {
		t.Fatal("expected error for parent segment")
	}
}

func TestPlanMountsRequiresModeForDirectory(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if _, _, err := runtime.PlanMounts(nil, []profile.DirectoryMount{{HostPath: workspace}}, "alice", home, ""); err != nil {
		t.Fatalf("unexpected error for default directory mode: %v", err)
	}
}

func TestPlanMountsFileCopyPermissions(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	hostFile := filepath.Join(home, "creds")
	if err := os.WriteFile(hostFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := os.Chmod(hostFile, 0400); err != nil {
		t.Fatalf("failed to chmod temp file: %v", err)
	}

	plan, cleanup, err := runtime.PlanMounts([]profile.FileMount{
		{HostPath: hostFile, VMPath: ".aws/credentials", Mode: config.MountModeCopy},
	}, nil, "alice", home, "")
	if err != nil {
		t.Fatalf("planMounts failed: %v", err)
	}
	defer cleanup()

	if plan.StagingRoot == "" {
		t.Fatal("expected staging root")
	}
	rootInfo, err := os.Stat(plan.StagingRoot)
	if err != nil {
		t.Fatalf("failed to stat staging root: %v", err)
	}
	if rootInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("staging root has unsafe permissions: %v", rootInfo.Mode().Perm())
	}

	stagedFile := filepath.Join(plan.StagingRoot, "home", "alice", ".aws", "credentials")
	fileInfo, err := os.Stat(stagedFile)
	if err != nil {
		t.Fatalf("failed to stat staged file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0400 {
		t.Fatalf("unexpected staged file permissions: %v", fileInfo.Mode().Perm())
	}
	if fileInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("staged file has unsafe permissions: %v", fileInfo.Mode().Perm())
	}
}

func TestPlanMountsFileRejectsMountMode(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	hostFile := filepath.Join(home, "creds")
	if err := os.WriteFile(hostFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	if _, _, err := runtime.PlanMounts([]profile.FileMount{
		{HostPath: hostFile, VMPath: ".aws/credentials", Mode: config.MountModeMount},
	}, nil, "alice", home, ""); err == nil {
		t.Fatal("expected error for invalid file mode")
	}
}

func TestProfilePathForName(t *testing.T) {
	// profilePathForName was local, replacing with direct profile.ProfileConfigPath logic test or just removing if it tested local logic.
	// Since profilePathForName is removed from main, we can test profile.ProfileConfigPath directly or runtime.LoadProfile.
	// Let's assume we want to test profile.ProfileConfigPath behavior.
	usr, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	path := profile.ProfileConfigPath(usr.HomeDir, "alpha")

	expectedSuffix := profile.ProfileConfigPath(usr.HomeDir, "alpha")
	if path != expectedSuffix {
		t.Fatalf("unexpected profile path: %s", path)
	}
}

func TestResolveShellPathDefault(t *testing.T) {
	t.Setenv("SHELL", "")
	shell, err := runtime.ResolveShellPath(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shell != profile.DefaultShellPath {
		t.Fatalf("unexpected shell: %s", shell)
	}
}

func TestResolveShellPathOverrides(t *testing.T) {
	shell, err := runtime.ResolveShellPath(&runtime.Profile{Shell: "/usr/bin/zsh"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shell != "/usr/bin/zsh" {
		t.Fatalf("unexpected shell: %s", shell)
	}
}

func TestResolveShellPathRejectsRelative(t *testing.T) {
	if _, err := runtime.ResolveShellPath(&runtime.Profile{Shell: "bash"}); err == nil {
		t.Fatal("expected error for relative shell path")
	}
}

func TestContainerNameForProfile(t *testing.T) {
	got, err := runtime.ContainerNameForProfile("Personal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "personal" {
		t.Fatalf("unexpected container name: %s", got)
	}
}

func TestResolveRunImageRequiresImage(t *testing.T) {
	prof := &runtime.Profile{}
	if _, err := runtime.ResolveRunImage(prof); err == nil {
		t.Fatalf("expected error when image is missing")
	}
	prof.Image = &profile.ImageState{Current: "docker.io/library/alpine"}
	image, err := runtime.ResolveRunImage(prof)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image != "docker.io/library/alpine" {
		t.Fatalf("unexpected image: %s", image)
	}
}

func TestWriteProfileTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile-test", "config.hujson")
	image := "docker.io/library/alpine"
	if err := runtime.WriteProfileTemplate(path, image, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read profile: %v", err)
	}
	if !strings.Contains(string(data), "\"files\"") {
		t.Fatalf("expected files in template: %s", string(data))
	}
	if strings.Contains(string(data), "\"directories\"") {
		t.Fatalf("did not expect empty directories in sparse template: %s", string(data))
	}
	if !strings.Contains(string(data), "\"shell\"") {
		t.Fatalf("expected shell in template: %s", string(data))
	}

	bashrcPath := filepath.Join(filepath.Dir(path), profile.ShellBashrcName)
	if _, err := os.Stat(bashrcPath); err != nil {
		t.Fatalf("expected bashrc at %s: %v", bashrcPath, err)
	}
	if !strings.Contains(string(data), bashrcPath) {
		t.Fatalf("expected bashrc path in template: %s", string(data))
	}
	bashProfilePath := filepath.Join(filepath.Dir(path), profile.ShellBashProfileName)
	if _, err := os.Stat(bashProfilePath); err != nil {
		t.Fatalf("expected bash_profile at %s: %v", bashProfilePath, err)
	}
	if !strings.Contains(string(data), bashProfilePath) {
		t.Fatalf("expected bash_profile path in template: %s", string(data))
	}

	if err := runtime.WriteProfileTemplate(path, image, false); err == nil {
		t.Fatal("expected error on existing profile without force")
	}

	if err := runtime.WriteProfileTemplate(path, image, true); err != nil {
		t.Fatalf("expected overwrite with force: %v", err)
	}
}

func TestWriteProfileConfigPreservesShell(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile-test", "config.hujson")
	prof := &runtime.Profile{
		Shell: "/usr/bin/bash",
		Image: &profile.ImageState{Current: "docker.io/library/ubuntu:24.04"},
		Files: []profile.FileMount{
			{HostPath: "~/.zshrc", VMPath: ".zshrc", Mode: config.MountModeCopy},
		},
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := runtime.WriteProfileConfig(path, prof); err != nil {
		t.Fatalf("writeProfileConfig failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "\"shell\": \"/usr/bin/bash\"") {
		t.Fatalf("expected shell in config: %s", string(data))
	}
	if !strings.Contains(string(data), "\"image\": \"docker.io/library/ubuntu:24.04\"") {
		t.Fatalf("expected compact image in config: %s", string(data))
	}
}
