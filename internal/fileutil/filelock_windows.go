//go:build windows

package fileutil

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// FileLock holds an exclusive advisory lock on a sibling .lock file.
type FileLock struct {
	file *os.File
}

// Close releases the lock and closes the underlying file. Unlock failures are
// returned to the caller after closing the file.
func (l *FileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	var ol windows.Overlapped
	if err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &ol); err != nil {
		_ = l.file.Close()
		return fmt.Errorf("fileutil: unlock file: %w", err)
	}
	return l.file.Close()
}

// LockSiblingFile acquires an exclusive advisory lock on the sibling file
// <path>.lock. The returned closer must be released by the caller.
func LockSiblingFile(path string) (*FileLock, error) {
	lockPath := siblingLockPath(path)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, fmt.Errorf("fileutil: mkdir lock dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("fileutil: open lock file: %w", err)
	}
	var ol windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("fileutil: lock file: %w", err)
	}
	return &FileLock{file: f}, nil
}

func siblingLockPath(path string) string {
	return path + ".lock"
}
