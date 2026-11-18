package runtime

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveContainerRuntimePathPrefersHomebrewFormula(t *testing.T) {
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "usr", "local", "bin", "container")
	brewPath := filepath.Join(tmpDir, "opt", "homebrew", "bin", "brew")
	brewPrefix := filepath.Join(tmpDir, "opt", "homebrew", "opt", "container")
	want := filepath.Join(brewPrefix, "bin", "container")

	got, err := resolveContainerRuntimePath("", func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return localPath, nil
		case ContainerBrewBinary:
			return brewPath, nil
		case want:
			return want, nil
		default:
			return "", exec.ErrNotFound
		}
	}, func(name string, args ...string) ([]byte, error) {
		if name == brewPath && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte(brewPrefix + "\n"), nil
		}
		return nil, errors.New("unexpected command")
	})
	if err != nil {
		t.Fatalf("resolveContainerRuntimePath error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected resolved path: got %q want %q", got, want)
	}
}

func TestResolveContainerRuntimePathFallsBackWhenBrewPrefixIsNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "usr", "local", "bin", "container")
	brewPath := filepath.Join(tmpDir, "opt", "homebrew", "bin", "brew")
	brewPrefix := filepath.Join(tmpDir, "opt", "homebrew", "opt", "container")

	got, err := resolveContainerRuntimePath("", func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return localPath, nil
		case ContainerBrewBinary:
			return brewPath, nil
		default:
			return "", exec.ErrNotFound
		}
	}, func(name string, args ...string) ([]byte, error) {
		if name == brewPath && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return []byte(brewPrefix + "\n"), nil
		}
		return nil, errors.New("unexpected command")
	})
	if err != nil {
		t.Fatalf("resolveContainerRuntimePath error: %v", err)
	}
	if got != localPath {
		t.Fatalf("unexpected resolved path: got %q want %q", got, localPath)
	}
}

func TestResolveContainerRuntimePathFallsBackToLocalWhenBrewFormulaMissing(t *testing.T) {
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "usr", "local", "bin", "container")
	brewPath := filepath.Join(tmpDir, "opt", "homebrew", "bin", "brew")

	got, err := resolveContainerRuntimePath("", func(name string) (string, error) {
		switch name {
		case ContainerRuntimeBinary:
			return localPath, nil
		case ContainerBrewBinary:
			return brewPath, nil
		default:
			return "", exec.ErrNotFound
		}
	}, func(name string, args ...string) ([]byte, error) {
		if name == brewPath && len(args) == 2 && args[0] == "--prefix" && args[1] == ContainerBrewFormula {
			return nil, errors.New("exit status 1")
		}
		return nil, errors.New("unexpected command")
	})
	if err != nil {
		t.Fatalf("resolveContainerRuntimePath error: %v", err)
	}
	if got != localPath {
		t.Fatalf("unexpected resolved path: got %q want %q", got, localPath)
	}
}
