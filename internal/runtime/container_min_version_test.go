package runtime

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/profile"
)

func TestResolveContainerBinRequiresMinimumVersion(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	binDir := t.TempDir()
	containerPath := filepath.Join(binDir, "container")
	if err := os.WriteFile(containerPath, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatalf("write container binary: %v", err)
	}
	t.Setenv("PATH", binDir)

	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name != containerPath {
			t.Fatalf("unexpected binary path: got %q want %q", name, containerPath)
		}
		if len(args) == 1 && args[0] == "--version" {
			return []byte("container version 1.5.9\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	_, err := ResolveContainerBin(profile.DefaultFlags())
	if err == nil {
		t.Fatalf("expected minimum-version error")
	}
	if !strings.Contains(err.Error(), "requires container 1.6.0 or newer") {
		t.Fatalf("expected minimum-version guidance, got: %v", err)
	}
}

func TestResolveContainerBinAcceptsSupportedVersion(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	binDir := t.TempDir()
	containerPath := filepath.Join(binDir, "container")
	if err := os.WriteFile(containerPath, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatalf("write container binary: %v", err)
	}
	t.Setenv("PATH", binDir)

	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name != containerPath {
			t.Fatalf("unexpected binary path: got %q want %q", name, containerPath)
		}
		if len(args) == 1 && args[0] == "--version" {
			return []byte("container version 1.6.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	containerServiceStatus = func(name, format string) ([]byte, error) {
		if name != containerPath {
			t.Fatalf("unexpected service-status binary path: got %q want %q", name, containerPath)
		}
		if format != "json" {
			t.Fatalf("unexpected service-status format: %q", format)
		}
		return []byte(`{"status":"running"}`), nil
	}
	containerServiceStart = func(out io.Writer, name string, opts RuntimeActionOptions) error {
		t.Fatalf("did not expect container service start for an already-running runtime")
		return nil
	}

	got, err := ResolveContainerBin(profile.DefaultFlags())
	if err != nil {
		t.Fatalf("ResolveContainerBin error: %v", err)
	}
	if got != containerPath {
		t.Fatalf("unexpected resolved path: got %q want %q", got, containerPath)
	}
}

func TestResolveContainerBinStartsContainerServiceWhenStopped(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	binDir := t.TempDir()
	containerPath := filepath.Join(binDir, "container")
	if err := os.WriteFile(containerPath, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatalf("write container binary: %v", err)
	}
	t.Setenv("PATH", binDir)

	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name != containerPath {
			t.Fatalf("unexpected binary path: got %q want %q", name, containerPath)
		}
		if len(args) == 1 && args[0] == "--version" {
			return []byte("container version 1.6.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	serviceStarted := false
	containerServiceStatus = func(name, format string) ([]byte, error) {
		if name != containerPath {
			t.Fatalf("unexpected service-status binary path: got %q want %q", name, containerPath)
		}
		if format != "json" {
			t.Fatalf("unexpected service-status format: %q", format)
		}
		return []byte(`{"status":"stopped"}`), nil
	}
	containerServiceStart = func(out io.Writer, name string, opts RuntimeActionOptions) error {
		if name != containerPath {
			t.Fatalf("unexpected service-start binary path: got %q want %q", name, containerPath)
		}
		serviceStarted = true
		return nil
	}

	got, err := ResolveContainerBin(profile.DefaultFlags())
	if err != nil {
		t.Fatalf("ResolveContainerBin error: %v", err)
	}
	if got != containerPath {
		t.Fatalf("unexpected resolved path: got %q want %q", got, containerPath)
	}
	if !serviceStarted {
		t.Fatalf("expected ResolveContainerBin to start the container service when it is stopped")
	}
}
