package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRunnerDirRequiresPath(t *testing.T) {
	if _, err := EnsureRunnerDir(" "); err == nil {
		t.Fatalf("expected error for empty staging root")
	}
}

func TestEnsureRunnerDirCreatesDir(t *testing.T) {
	base := t.TempDir()
	path, err := EnsureRunnerDir(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "runner")
	if path != want {
		t.Fatalf("expected %s, got %s", want, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected runner path to be a directory")
	}
	if got := info.Mode().Perm(); got != 0700 {
		t.Fatalf("expected mode 0700, got %04o", got)
	}
}
