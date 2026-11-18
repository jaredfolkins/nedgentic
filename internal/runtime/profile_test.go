package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/config"
	"nedgentic/internal/profile"
)

func TestLoadProfileRejectsHostAgentAuthImports(t *testing.T) {
	for _, imports := range []string{
		`"directories": [{"host": "~/.codex", "vm": ".codex", "mode": "copy"}]`,
		`"directories": [{"host": "~/.claude", "vm": ".claude", "mode": "copy"}]`,
		`"files": [{"host": "~/.claude.json", "vm": ".claude.json"}]`,
		`"files": [{"host": "~/.grok/auth.json", "vm": ".grok/auth.json"}]`,
		`"directories": [{"host": "~/.local/share/opencode", "vm": ".local/share/opencode", "mode": "copy"}]`,
	} {
		path := filepath.Join(t.TempDir(), "config.hujson")
		if err := os.WriteFile(path, []byte(`{"imports": {`+imports+`}}`), 0600); err != nil {
			t.Fatalf("write profile: %v", err)
		}
		_, err := LoadProfile(path, true, "default")
		if err == nil || !strings.Contains(err.Error(), "imports host agent logins") {
			t.Fatalf("expected host agent login import to fail for %s, got %v", imports, err)
		}
	}
}

func TestLoadProfileRejectsToolsImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.hujson")
	if err := os.WriteFile(path, []byte(`{"imports": {"tools": ["codex"]}}`), 0600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	_, err := LoadProfile(path, true, "default")
	if err == nil || !strings.Contains(err.Error(), "imports.tools was removed") || !strings.Contains(err.Error(), "ned-agent-login") {
		t.Fatalf("expected removed tools import to fail with guidance, got %v", err)
	}
}

func TestLoadProfileRejectsEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	if err := os.WriteFile(path, []byte("{\"env\": {\"FOO\": \"bar\"}}"), 0600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	if _, err := LoadProfile(path, true, "default"); err == nil {
		t.Fatalf("expected error for rejected env block")
	} else if !strings.Contains(err.Error(), `unknown field "env"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteProfileConfigDefaultsToMinimalImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-test", "config.hujson")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := WriteProfileConfig(path, nil); err != nil {
		t.Fatalf("WriteProfileConfig returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"image": "`+profile.DefaultImageReference()+`"`) {
		t.Fatalf("expected minimal default image in config, got:\n%s", text)
	}
}

func TestLoadGeneratedDefaultProfileUsesMinimalImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-test", "config.hujson")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := WriteProfileConfig(path, nil); err != nil {
		t.Fatalf("WriteProfileConfig returned error: %v", err)
	}

	prof, err := LoadProfile(path, true, "default")
	if err != nil {
		t.Fatalf("LoadProfile returned error: %v", err)
	}
	if prof.Image == nil || prof.Image.Current != profile.DefaultImageReference() {
		t.Fatalf("expected default image %q, got %#v", profile.DefaultImageReference(), prof.Image)
	}
	image, err := ResolveRunImage(prof)
	if err != nil {
		t.Fatalf("ResolveRunImage returned error: %v", err)
	}
	if image != profile.DefaultImageReference() {
		t.Fatalf("expected run image %q, got %q", profile.DefaultImageReference(), image)
	}
}

func TestCompactProfileFields(t *testing.T) {
	content := []byte(`{
  "shell": "/bin/bash",
  "image": "docker.io/library/alpine",
  "resources": {
    "cpus": "3",
    "memory": "6G"
  }
}`)
	prof, err := ParseProfileConfig(content)
	if err != nil {
		t.Fatalf("parse compact profile: %v", err)
	}
	flags := EffectiveFlags(&prof)
	if flags.CPUs != "3" || flags.Memory != "6G" {
		t.Fatalf("expected resources to set flags, got cpus=%q memory=%q", flags.CPUs, flags.Memory)
	}
	if prof.Image == nil || prof.Image.Current != "docker.io/library/alpine" {
		t.Fatalf("expected compact image to parse, got %#v", prof.Image)
	}
}

func TestGroupedProfilePrimitivesNormalizeToRuntimeFields(t *testing.T) {
	content := []byte(`{
  "shell": "/bin/bash",
  "image": "docker.io/library/alpine",
  "resources": {
    "cpus": "3",
    "memory": "6G"
  },
  "imports": {
    "binaries": ["rg"]
  }
}`)
	path := filepath.Join(t.TempDir(), "config.hujson")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	prof, err := LoadProfile(path, true, "default")
	if err != nil {
		t.Fatalf("LoadProfile error: %v", err)
	}
	if strings.Join(prof.Binaries, ",") != "rg" {
		t.Fatalf("expected grouped imports binaries, got %#v", prof.Binaries)
	}
}

func TestParseProfileConfigRejectsCapabilities(t *testing.T) {
	for _, key := range []string{"observability", "neduardian", "host-exec"} {
		_, err := ParseProfileConfig([]byte(`{"capabilities": {"` + key + `": true}}`))
		if err == nil || !strings.Contains(err.Error(), `unknown field "capabilities"`) {
			t.Fatalf("expected capabilities.%s to be rejected, got %v", key, err)
		}
	}
}

func TestLoadProfileRejectsMixedGroupedAndLegacyImports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.hujson")
	content := []byte(`{
  "imports": {
    "binaries": ["rg"]
  },
  "binaries": ["rg"]
}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	if _, err := LoadProfile(path, true, "default"); err == nil {
		t.Fatalf("expected legacy top-level imports shape to fail")
	} else if !strings.Contains(err.Error(), `unknown field "binaries"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertFileMount(t *testing.T, files []profile.FileMount, host, vm string) {
	t.Helper()
	for _, file := range files {
		if file.HostPath == host && file.VMPath == vm {
			if file.Mode != config.MountModeCopy {
				t.Fatalf("expected %s to mount with mode %q, got %q", host, config.MountModeCopy, file.Mode)
			}
			return
		}
	}
	t.Fatalf("expected file mount host=%q vm=%q to be present", host, vm)
}

func assertDirectoryMount(t *testing.T, dirs []profile.DirectoryMount, host, vm string) {
	t.Helper()
	for _, dir := range dirs {
		if dir.HostPath == host && dir.VMPath == vm {
			if dir.Mode != config.MountModeCopy {
				t.Fatalf("expected %s to mount with mode %q, got %q", host, config.MountModeCopy, dir.Mode)
			}
			return
		}
	}
	t.Fatalf("expected directory mount host=%q vm=%q to be present", host, vm)
}

func assertNoBinaryMount(t *testing.T, files []profile.FileMount, name string) {
	t.Helper()
	target := "/usr/local/bin/" + name
	for _, file := range files {
		if file.VMPath == target {
			t.Fatalf("expected no binary mount for %q, found host %q", name, file.HostPath)
		}
	}
}

// Profiles are edited by hand, so the README's example must parse.
func TestReadmeProfileExampleParses(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	_, section, ok := strings.Cut(string(content), "## Profiles\n")
	if !ok {
		t.Fatal("README has no Profiles section")
	}
	_, block, ok := strings.Cut(section, "```json\n")
	if !ok {
		t.Fatal("Profiles section has no json example")
	}
	example, _, ok := strings.Cut(block, "```")
	if !ok {
		t.Fatal("unterminated json example")
	}
	loaded, err := ParseProfileConfig([]byte(example))
	if err != nil {
		t.Fatalf("example profile does not parse: %v", err)
	}
	if err := rejectHostAgentAuthImports(&loaded); err != nil {
		t.Fatalf("example profile is rejected: %v", err)
	}
	if loaded.Image == nil || loaded.Image.Current == "" || len(loaded.Files) != 1 || len(loaded.Directories) != 1 {
		t.Fatalf("unexpected example profile: %+v", loaded)
	}
}
