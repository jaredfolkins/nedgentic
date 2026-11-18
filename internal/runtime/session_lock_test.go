package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nedgentic/internal/profile"
)

func TestSessionLockUpgrade(t *testing.T) {
	containerName := "session-lock-test"
	lock1, err := AcquireSessionLock(containerName)
	if err != nil {
		t.Fatalf("AcquireSessionLock lock1: %v", err)
	}

	lock2, err := AcquireSessionLock(containerName)
	if err != nil {
		t.Fatalf("AcquireSessionLock lock2: %v", err)
	}
	if ok, err := lock1.TryUpgradeExclusive(); err != nil || ok {
		t.Fatalf("expected upgrade to fail with active shared lock; ok=%v err=%v", ok, err)
	}
	if err := lock2.Release(); err != nil {
		t.Fatalf("release lock2: %v", err)
	}
	if ok, err := lock1.TryUpgradeExclusive(); err != nil || !ok {
		t.Fatalf("expected upgrade to succeed after release; ok=%v err=%v", ok, err)
	}
	if err := lock1.Release(); err != nil {
		t.Fatalf("release lock1: %v", err)
	}

	cleanupSessionDir(t, containerName)
}

func cleanupSessionDir(t *testing.T, containerName string) {
	t.Helper()
	home, err := profile.ResolveHome()
	if err != nil {
		t.Fatalf("ResolveHome: %v", err)
	}
	safe := strings.ReplaceAll(containerName, string(filepath.Separator), "-")
	path := filepath.Join(profile.SessionsDir(home), safe)
	_ = os.RemoveAll(path)
}
