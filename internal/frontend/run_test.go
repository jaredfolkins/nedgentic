package frontend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/profile"
)

func TestGenticCommandHookDirFlagWiring(t *testing.T) {
	if genticCmd.PersistentFlags().Lookup("hook-dir") == nil {
		t.Fatalf("expected --hook-dir flag on gentic command")
	}
}

func TestResolveGenticHookSourcePrefersHookDir(t *testing.T) {
	home := t.TempDir()
	profileDir := filepath.Join(home, "profile")
	if err := os.MkdirAll(filepath.Join(profileDir, "gentic"), 0700); err != nil {
		t.Fatalf("create profile gentic dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "gentic", "00-profile.sh"), []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write profile hook: %v", err)
	}
	hookDir := filepath.Join(home, "hooks")
	if err := os.MkdirAll(hookDir, 0700); err != nil {
		t.Fatalf("create hook dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hookDir, "00-dev.sh"), []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write dev hook: %v", err)
	}

	got, force, err := resolveGenticHookSource(profileDir, hookDir, home)
	if err != nil {
		t.Fatalf("resolveGenticHookSource error: %v", err)
	}
	want, err := resolveHookPath(hookDir)
	if err != nil {
		t.Fatalf("resolveHookPath error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected hook dir: got %q want %q", got, want)
	}
	if !force {
		t.Fatalf("expected explicit hook dir to force mounted hook usage")
	}
}

func TestResolveGenticHookSourceFallsBackToProfileHooks(t *testing.T) {
	home := t.TempDir()
	profileDir := filepath.Join(home, "profile")
	profileHookDir := filepath.Join(profileDir, "gentic")
	if err := os.MkdirAll(profileHookDir, 0700); err != nil {
		t.Fatalf("create profile gentic dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileHookDir, "00-profile.sh"), []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write profile hook: %v", err)
	}

	got, force, err := resolveGenticHookSource(profileDir, "", home)
	if err != nil {
		t.Fatalf("resolveGenticHookSource error: %v", err)
	}
	if got != profileHookDir {
		t.Fatalf("unexpected profile hook dir: got %q want %q", got, profileHookDir)
	}
	if force {
		t.Fatalf("did not expect profile hook dir to force mounted hook usage")
	}
}

func TestResolveHookDirRejectsOutsideHome(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "00-dev.sh"), []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write dev hook: %v", err)
	}

	_, err := resolveHookDir(outside, home)
	if err == nil {
		t.Fatal("expected outside-home hook dir to fail")
	}
	if !strings.Contains(err.Error(), "under host home") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveHookDirRequiresShellHooks(t *testing.T) {
	home := t.TempDir()
	hookDir := filepath.Join(home, "hooks")
	if err := os.MkdirAll(hookDir, 0700); err != nil {
		t.Fatalf("create hook dir: %v", err)
	}

	_, err := resolveHookDir(hookDir, home)
	if err == nil {
		t.Fatal("expected empty hook dir to fail")
	}
	if !strings.Contains(err.Error(), "at least one .sh hook") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLaunchFlagsAllowsContainerSystemd(t *testing.T) {
	flags := profile.DefaultFlags()
	enabled := true
	flags.Systemd = &enabled
	if err := validateLaunchFlags(flags); err != nil {
		t.Fatalf("validateLaunchFlags error: %v", err)
	}
}

func TestValidateLaunchFlagsRejectsInitConflicts(t *testing.T) {
	flags := profile.DefaultFlags()
	systemd := true
	init := true
	flags.Systemd = &systemd
	flags.Init = &init
	err := validateLaunchFlags(flags)
	if err == nil {
		t.Fatal("expected systemd/init conflict to fail")
	}
	if !strings.Contains(err.Error(), "--systemd cannot be combined with --init") {
		t.Fatalf("unexpected error: %v", err)
	}

	flags.Init = nil
	flags.InitImage = "docker.io/library/custom-init:latest"
	err = validateLaunchFlags(flags)
	if err == nil {
		t.Fatal("expected systemd/init-image conflict to fail")
	}
	if !strings.Contains(err.Error(), "--systemd cannot be combined with --init-image") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintContainerStartupLogsTrimsAndWritesFetchedLogs(t *testing.T) {
	var out strings.Builder
	ok := printContainerStartupLogs(&out, func(bin, name string) (string, error) {
		if bin != "container" {
			t.Fatalf("unexpected bin: %q", bin)
		}
		if name != "vm-name" {
			t.Fatalf("unexpected container name: %q", name)
		}
		return "\nRun Gentic script [4/4]: /ned/hooks/03-agent-updates.sh [0.100s]\n\n", nil
	}, "container", "vm-name")

	if !ok {
		t.Fatal("expected startup logs to be printed")
	}
	if got, want := out.String(), "Run Gentic script [4/4]: /ned/hooks/03-agent-updates.sh [0.100s]\n"; got != want {
		t.Fatalf("unexpected logs:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPrintContainerStartupLogsSkipsEmptyLogs(t *testing.T) {
	var out strings.Builder
	ok := printContainerStartupLogs(&out, func(_, _ string) (string, error) {
		return " \n\t", nil
	}, "container", "vm-name")

	if ok {
		t.Fatal("did not expect empty startup logs to be printed")
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
