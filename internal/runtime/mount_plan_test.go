package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/config"
	"nedgentic/internal/profile"
)

func TestMapHostPathToGuest(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	project := filepath.Join(hostHome, "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	got, ok, err := MapHostPathToGuest(project, hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected mapping to be ok")
	}
	if got != "/home/alice/project" {
		t.Fatalf("expected /home/alice/project, got %s", got)
	}

	got, ok, err = MapHostPathToGuest(hostHome, hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected mapping to be ok for home dir")
	}
	if got != "/home/alice" {
		t.Fatalf("expected /home/alice, got %s", got)
	}

	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	got, ok, err = MapHostPathToGuest(outside, hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected mapping to be false for outside path")
	}
	if got != "" {
		t.Fatalf("expected empty mapping for outside path, got %s", got)
	}

	got, ok, err = MapHostPathToGuest(outside, "", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected mapping to be false for empty home")
	}
	if got != "" {
		t.Fatalf("expected empty mapping for empty home, got %s", got)
	}
}

func TestResolveTargetPath(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	project := filepath.Join(hostHome, "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	got, err := ResolveTargetPath(project, "", hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/alice/project" {
		t.Fatalf("expected /home/alice/project, got %s", got)
	}

	got, err = ResolveTargetPath(outside, "", hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != outside {
		t.Fatalf("expected %s, got %s", outside, got)
	}

	got, err = ResolveTargetPath(outside, "~/config", hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/alice/config" {
		t.Fatalf("expected /home/alice/config, got %s", got)
	}

	got, err = ResolveTargetPath(outside, "/var/log", hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/var/log" {
		t.Fatalf("expected /var/log, got %s", got)
	}

	got, err = ResolveTargetPath(outside, "work", hostHome, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/alice/work" {
		t.Fatalf("expected /home/alice/work, got %s", got)
	}

	if _, err := ResolveTargetPath(outside, "../bad", hostHome, "alice"); err == nil {
		t.Fatalf("expected error for traversal vm path")
	}
	if _, err := ResolveTargetPath(outside, "work/../bad", hostHome, "alice"); err == nil {
		t.Fatalf("expected error for nested traversal vm path")
	}
}

func TestPathValidationHelpers(t *testing.T) {
	if err := validateContainerPath("ok/path"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateContainerPath("bad,path"); err == nil {
		t.Fatalf("expected error for comma in path")
	}

	home := t.TempDir()
	homeInside := filepath.Join(home, "proj")
	if err := os.MkdirAll(homeInside, 0700); err != nil {
		t.Fatalf("mkdir inside: %v", err)
	}
	if err := ensureUnderHome(homeInside, home); err != nil {
		t.Fatalf("unexpected error for home path: %v", err)
	}
	homeOutside := filepath.Join(filepath.Dir(home), "other")
	if err := ensureUnderHome(homeOutside, home); err == nil {
		t.Fatalf("expected error for path outside home")
	}
	link := filepath.Join(home, "link")
	if err := os.Symlink(homeOutside, link); err == nil {
		if err := ensureUnderHome(link, home); err == nil {
			t.Fatalf("expected error for symlink outside home")
		}
	}

	root := t.TempDir()
	if err := ensureUnderRoot(root, root); err == nil {
		t.Fatalf("expected error when dest resolves to root")
	}
	rootInside := filepath.Join(root, "file")
	if err := ensureUnderRoot(root, rootInside); err != nil {
		t.Fatalf("unexpected error for inside path: %v", err)
	}
	rootOutside := filepath.Join(filepath.Dir(root), "other")
	if err := ensureUnderRoot(root, rootOutside); err == nil {
		t.Fatalf("expected error for outside path")
	}

	if containsParentSegment("a/b") {
		t.Fatalf("expected no parent segment in a/b")
	}
	if !containsParentSegment("a/../b") {
		t.Fatalf("expected parent segment in a/../b")
	}
	if containsParentSegment("a/..b") {
		t.Fatalf("did not expect parent segment in a/..b")
	}
	pathWithParent := "a" + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "b"
	if !containsParentSegment(pathWithParent) {
		t.Fatalf("expected parent segment in %s", pathWithParent)
	}
}

func TestPlanMountsWorkspaceAndFileCopy(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "project")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	secretDir := filepath.Join(hostHome, "secrets")
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		t.Fatalf("mkdir secrets: %v", err)
	}
	secretFile := filepath.Join(secretDir, "token")
	if err := os.WriteFile(secretFile, []byte("secret"), 0600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	files := []profile.FileMount{
		{HostPath: secretFile, VMPath: "/etc/ned/token"},
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	plan, cleanup, err := PlanMounts(files, nil, "alice", hostHome, "")
	if err != nil {
		t.Fatalf("PlanMounts error: %v", err)
	}
	t.Cleanup(cleanup)

	if plan.WorkspacePath != "/home/alice/project" {
		t.Fatalf("expected workspace path /home/alice/project, got %s", plan.WorkspacePath)
	}
	expectedMount := workspaceMountPath(cwd)
	if plan.WorkspaceMount != expectedMount {
		t.Fatalf("expected workspace mount %s, got %s", expectedMount, plan.WorkspaceMount)
	}
	if plan.StagingRoot == "" {
		t.Fatalf("expected staging root to be set")
	}
	if len(plan.CopySpecs) != 1 {
		t.Fatalf("expected 1 copy spec, got %d", len(plan.CopySpecs))
	}
	spec := plan.CopySpecs[0]
	if spec.DestPath != "/etc/ned/token" {
		t.Fatalf("expected dest /etc/ned/token, got %s", spec.DestPath)
	}
	if spec.SourceRel != "etc/ned/token" {
		t.Fatalf("expected source rel etc/ned/token, got %s", spec.SourceRel)
	}
	if spec.Mode.Perm() != 0600 {
		t.Fatalf("expected mode 0600, got %04o", spec.Mode.Perm())
	}

	var workspaceFound bool
	var secretsFound bool
	for _, mount := range plan.Mounts {
		switch mount.Target {
		case expectedMount:
			workspaceFound = true
			if mount.Source != cwd {
				t.Fatalf("expected workspace source %s, got %s", cwd, mount.Source)
			}
		case SecretsMountPath:
			secretsFound = true
			if !mount.ReadOnly {
				t.Fatalf("expected secrets mount to be read-only")
			}
		}
	}
	if !workspaceFound {
		t.Fatalf("expected workspace mount to be present")
	}
	if !secretsFound {
		t.Fatalf("expected secrets mount to be present")
	}
}

func TestPlanMountsFileCopyClampsPermissions(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	hostFile := filepath.Join(hostHome, "creds")
	if err := os.WriteFile(hostFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	plan, cleanup, err := PlanMounts([]profile.FileMount{
		{HostPath: hostFile, VMPath: ".aws/credentials"},
	}, nil, "alice", hostHome, "")
	if err != nil {
		t.Fatalf("PlanMounts error: %v", err)
	}
	t.Cleanup(cleanup)

	if plan.StagingRoot == "" {
		t.Fatalf("expected staging root to be set")
	}
	stagedFile := filepath.Join(plan.StagingRoot, "home", "alice", ".aws", "credentials")
	info, err := os.Stat(stagedFile)
	if err != nil {
		t.Fatalf("failed to stat staged file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected staged file permissions: %v", info.Mode().Perm())
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("staged file has unsafe permissions: %v", info.Mode().Perm())
	}
}

func TestPlanMountsDirectoryCopyClampsPermissions(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	sourceDir := filepath.Join(hostHome, "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "nested"), 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	sourceFile := filepath.Join(sourceDir, "nested", "config.toml")
	if err := os.WriteFile(sourceFile, []byte("token = \"x\"\n"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	plan, cleanup, err := PlanMounts(nil, []profile.DirectoryMount{
		{HostPath: sourceDir, VMPath: ".config/tool", Mode: config.MountModeCopy},
	}, "alice", hostHome, "")
	if err != nil {
		t.Fatalf("PlanMounts error: %v", err)
	}
	t.Cleanup(cleanup)

	stagedDir := filepath.Join(plan.StagingRoot, "home", "alice", ".config", "tool", "nested")
	dirInfo, err := os.Stat(stagedDir)
	if err != nil {
		t.Fatalf("stat staged dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("unexpected staged dir permissions: %04o", dirInfo.Mode().Perm())
	}

	stagedFile := filepath.Join(stagedDir, "config.toml")
	fileInfo, err := os.Stat(stagedFile)
	if err != nil {
		t.Fatalf("stat staged file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("unexpected staged file permissions: %04o", fileInfo.Mode().Perm())
	}
}

func TestPlanMountsRejectsWorkspaceOutsideHome(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	if err := os.MkdirAll(hostHome, 0700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	workspace := filepath.Join(base, "workspace")
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
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if _, _, err := PlanMounts(nil, nil, "alice", hostHome, ""); err == nil {
		t.Fatalf("expected error for workspace outside home")
	}
}

func TestPlanMountsRejectsMissingExplicitFileImport(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
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
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	missing := filepath.Join(hostHome, "missing.env")
	if _, _, err := PlanMounts([]profile.FileMount{
		{HostPath: missing, VMPath: ".env"},
	}, nil, "alice", hostHome, ""); err == nil {
		t.Fatalf("expected missing explicit file import to fail")
	}
}

func TestPlanMountsRejectsDirectoryOutsideHome(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	dirs := []profile.DirectoryMount{
		{HostPath: outside, VMPath: "/opt/outside", Mode: "mount", Access: "readonly"},
	}
	if _, _, err := PlanMounts(nil, dirs, "alice", hostHome, ""); err == nil {
		t.Fatalf("expected error for directory outside home")
	}
}

func TestPlanMountsRejectsCopyDirectoryWithAccess(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
	if err := os.MkdirAll(hostHome, 0700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	dir := filepath.Join(hostHome, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	_, _, err = PlanMounts(nil, []profile.DirectoryMount{
		{HostPath: dir, VMPath: ".claude", Mode: config.MountModeCopy, Access: config.AccessReadWrite},
	}, "alice", hostHome, "")
	if err == nil {
		t.Fatalf("expected error for copy directory with access set")
	}
}

func TestPlanMountsRejectsStagingRootOutsideHome(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	hostFile := filepath.Join(hostHome, "token")
	if err := os.WriteFile(hostFile, []byte("secret"), 0600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	stagingRoot := filepath.Join(base, "staging-outside")
	if err := os.MkdirAll(stagingRoot, 0700); err != nil {
		t.Fatalf("mkdir staging root: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	_, cleanup, err := PlanMounts([]profile.FileMount{
		{HostPath: hostFile, VMPath: ".aws/credentials"},
	}, nil, "alice", hostHome, stagingRoot)
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatalf("expected error for staging root outside home")
	}
	if !strings.Contains(err.Error(), "staging root must be under host home") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanMountsRejectsStagingSymlinkForFileCopy(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	hostFile := filepath.Join(hostHome, "token")
	if err := os.WriteFile(hostFile, []byte("secret"), 0600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	stagingRoot := filepath.Join(hostHome, ".ned", "staging", "manual")
	if err := os.MkdirAll(stagingRoot, 0700); err != nil {
		t.Fatalf("mkdir staging root: %v", err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(stagingRoot, "home")); err != nil {
		t.Fatalf("symlink staging: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	_, cleanup, err := PlanMounts([]profile.FileMount{
		{HostPath: hostFile, VMPath: "/home/alice/.aws/credentials"},
	}, nil, "alice", hostHome, stagingRoot)
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatalf("expected error for staging symlink")
	}
	if !strings.Contains(err.Error(), "staging path contains symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyDirRejectsSymlinkDestination(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "staging")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	src := filepath.Join(base, "src")
	if err := os.MkdirAll(filepath.Join(src, "subdir"), 0700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "subdir", "file.txt"), []byte("data"), 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(root, "opt", "data")
	if err := os.MkdirAll(filepath.Join(dst), 0700); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dst, "subdir")); err != nil {
		t.Fatalf("symlink dst: %v", err)
	}

	if err := copyDir(root, src, dst); err == nil {
		t.Fatalf("expected symlink error")
	} else if !strings.Contains(err.Error(), "staging path contains symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyDirFiltersOnlyKnownAgentRoots(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	src := filepath.Join(base, "source")
	dst := filepath.Join(root, "home", "alice", "copied")

	mustWriteFile(t, filepath.Join(src, "history.jsonl"), "history")
	mustWriteFile(t, filepath.Join(src, "cache", "blob"), "cache")

	if err := copyDir(root, src, dst); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	assertPathExists(t, filepath.Join(dst, "history.jsonl"))
	assertPathExists(t, filepath.Join(dst, "cache", "blob"))
}

func TestPlanMountsDirectoryMountsAndDuplicates(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "project")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	dirOne := filepath.Join(hostHome, "dir1")
	if err := os.MkdirAll(dirOne, 0700); err != nil {
		t.Fatalf("mkdir dir1: %v", err)
	}
	dirTwo := filepath.Join(hostHome, "dir2")
	if err := os.MkdirAll(dirTwo, 0700); err != nil {
		t.Fatalf("mkdir dir2: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	dirs := []profile.DirectoryMount{
		{HostPath: dirOne, VMPath: "/opt/data", Mode: "mount", Access: "readonly"},
		{HostPath: dirTwo, VMPath: "/opt/data", Mode: "mount", Access: "readwrite"},
	}
	plan, cleanup, err := PlanMounts(nil, dirs, "alice", hostHome, "")
	if err != nil {
		t.Fatalf("PlanMounts error: %v", err)
	}
	t.Cleanup(cleanup)

	if plan.StagingRoot != "" {
		t.Fatalf("expected no staging root for mount-only plan")
	}
	if len(plan.CopySpecs) != 0 {
		t.Fatalf("expected no copy specs, got %d", len(plan.CopySpecs))
	}

	var dataMounts []ContainerMount
	for _, mount := range plan.Mounts {
		if mount.Target == "/opt/data" {
			dataMounts = append(dataMounts, mount)
		}
	}
	if len(dataMounts) != 1 {
		t.Fatalf("expected 1 data mount, got %d", len(dataMounts))
	}
	if dataMounts[0].Source != dirOne {
		t.Fatalf("expected first mount source %s, got %s", dirOne, dataMounts[0].Source)
	}
	if !dataMounts[0].ReadOnly {
		t.Fatalf("expected data mount to be read-only")
	}
}

func TestPlanMountsAccessValidation(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "project")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	dir := filepath.Join(hostHome, "dir")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	dirs := []profile.DirectoryMount{
		{HostPath: dir, VMPath: "/opt/data", Mode: "mount", Access: "nope"},
	}
	if _, _, err := PlanMounts(nil, dirs, "alice", hostHome, ""); err == nil {
		t.Fatalf("expected error for invalid access")
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, stat err=%v", path, err)
	}
}

func TestPlanMountsRejectsCommaInDirectorySource(t *testing.T) {
	base := t.TempDir()
	hostHome := filepath.Join(base, "home")
	workspace := filepath.Join(hostHome, "project")
	injected := filepath.Join(hostHome, "x,target=/etc,readonly=false")
	for _, dir := range []string{workspace, injected} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	dirs := []profile.DirectoryMount{{HostPath: injected, VMPath: "data", Mode: config.MountModeMount}}
	_, cleanup, err := PlanMounts(nil, dirs, "alice", hostHome, "")
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	if err == nil || !strings.Contains(err.Error(), "commas") {
		t.Fatalf("expected comma rejection for directory mount source, got %v", err)
	}
}
