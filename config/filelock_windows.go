//go:build windows

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type fileLock struct {
	file *os.File
}

func lockConfigFile(path string) (*fileLock, error) {
	lockPath := siblingLockPath(path)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, fmt.Errorf("config: mkdir lock dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("config: open lock file: %w", err)
	}
	var ol windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("config: lock file: %w", err)
	}
	return &fileLock{file: f}, nil
}

func (l *fileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	var ol windows.Overlapped
	if err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &ol); err != nil {
		_ = l.file.Close()
		return fmt.Errorf("config: unlock file: %w", err)
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("config: close lock file: %w", err)
	}
	return nil
}

func siblingLockPath(path string) string {
	return path + ".lock"
}
