package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"nedgentic/internal/profile"
)

type SessionLock struct {
	path string
	file *os.File
}

func AcquireSessionLock(containerName string) (*SessionLock, error) {
	lockPath, err := sessionLockPath(containerName)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &SessionLock{path: lockPath, file: file}, nil
}

func (l *SessionLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *SessionLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *SessionLock) TryUpgradeExclusive() (bool, error) {
	if l == nil || l.file == nil {
		return false, fmt.Errorf("session lock is required")
	}
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func sessionLockPath(containerName string) (string, error) {
	name := strings.TrimSpace(containerName)
	if name == "" {
		return "", fmt.Errorf("container name is required")
	}
	home, err := profile.ResolveHome()
	if err != nil {
		return "", err
	}
	safe := strings.ReplaceAll(name, string(filepath.Separator), "-")
	lockDir := filepath.Join(profile.SessionsDir(home), safe)
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(lockDir, "lock"), nil
}
