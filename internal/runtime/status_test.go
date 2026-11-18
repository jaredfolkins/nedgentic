package runtime

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestWriteRuntimeStatusMissing(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	statusLookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}
	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		t.Fatalf("unexpected command output call: %s %v", name, args)
		return nil, nil
	}

	var out bytes.Buffer
	if err := WriteRuntimeStatus(&out); err != nil {
		t.Fatalf("WriteRuntimeStatus error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "container: not found") {
		t.Fatalf("expected container not found output, got: %s", output)
	}
	if !strings.Contains(output, "install: ned runtime install") {
		t.Fatalf("expected install hint output, got: %s", output)
	}
	if !strings.Contains(output, "homebrew: not found") {
		t.Fatalf("expected homebrew not found output, got: %s", output)
	}
}

func TestWriteRuntimeStatusInstalled(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	statusLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "/usr/local/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		case "/opt/homebrew/opt/container/bin/container":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte("/opt/homebrew/opt/container\n"), nil
		}
		if name == "/opt/homebrew/opt/container/bin/container" && len(args) == 1 && args[0] == "--version" {
			return []byte("container version 0.10.0\n"), nil
		}
		if name == "/opt/homebrew/bin/brew" && len(args) == 3 && args[0] == "list" && args[1] == "--versions" && args[2] == ContainerBrewFormula {
			return []byte("container 0.10.0\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	var out bytes.Buffer
	if err := WriteRuntimeStatus(&out); err != nil {
		t.Fatalf("WriteRuntimeStatus error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "container path: /opt/homebrew/opt/container/bin/container") {
		t.Fatalf("expected container path, got: %s", output)
	}
	if !strings.Contains(output, "container version: container version 0.10.0") {
		t.Fatalf("expected container version, got: %s", output)
	}
	if !strings.Contains(output, "homebrew path: /opt/homebrew/bin/brew") {
		t.Fatalf("expected homebrew path, got: %s", output)
	}
	if !strings.Contains(output, "homebrew container: container 0.10.0") {
		t.Fatalf("expected brew container version, got: %s", output)
	}
}

func TestWriteRuntimeStatusBrewNotInstalled(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	statusLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "", exec.ErrNotFound
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return nil, errors.New("exit status 1")
		}
		if name == "/opt/homebrew/bin/brew" && len(args) == 3 && args[0] == "list" && args[1] == "--versions" && args[2] == ContainerBrewFormula {
			return nil, errors.New("exit status 1")
		}
		return nil, errors.New("unexpected command")
	}

	var out bytes.Buffer
	if err := WriteRuntimeStatus(&out); err != nil {
		t.Fatalf("WriteRuntimeStatus error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "homebrew container: not found") {
		t.Fatalf("expected brew not found output, got: %s", output)
	}
}

func TestWriteRuntimeStatusBrewMissingButContainerInstalled(t *testing.T) {
	restore := saveStatusDeps()
	defer restore()

	statusLookPath = func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return "/usr/local/bin/container", nil
		case ContainerBrewBinary:
			return "/opt/homebrew/bin/brew", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	statusCommandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "/opt/homebrew/bin/brew" && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return nil, errors.New("exit status 1")
		}
		if name == "/usr/local/bin/container" && len(args) == 1 && args[0] == "--version" {
			return []byte("container version 0.10.0\n"), nil
		}
		if name == "/opt/homebrew/bin/brew" && len(args) == 3 && args[0] == "list" && args[1] == "--versions" && args[2] == ContainerBrewFormula {
			return nil, errors.New("exit status 1")
		}
		return nil, errors.New("unexpected command")
	}

	var out bytes.Buffer
	if err := WriteRuntimeStatus(&out); err != nil {
		t.Fatalf("WriteRuntimeStatus error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "homebrew container: <installed on system>") {
		t.Fatalf("expected installed-on-system output, got: %s", output)
	}
}

func saveStatusDeps() func() {
	origLookPath := statusLookPath
	origCommandOutput := statusCommandOutput
	origServiceStatus := containerServiceStatus
	origServiceStart := containerServiceStart
	origServiceReady := make(map[string]bool, len(containerServiceReady))
	for key, value := range containerServiceReady {
		origServiceReady[key] = value
	}
	return func() {
		statusLookPath = origLookPath
		statusCommandOutput = origCommandOutput
		containerServiceStatus = origServiceStatus
		containerServiceStart = origServiceStart
		containerServiceReady = origServiceReady
	}
}
