package fileutil

import (
	"os"
	"path/filepath"
)

type AtomicWriteOptions struct {
	FileMode         os.FileMode
	DirMode          os.FileMode
	ChmodExistingDir bool
	TempPattern      string
	Rename           func(string, string) error
	SyncDir          bool
}

func AtomicWriteFile(path string, data []byte, opts AtomicWriteOptions) error {
	fileMode := opts.FileMode
	if fileMode == 0 {
		fileMode = 0o600
	}
	dirMode := opts.DirMode
	if dirMode == 0 {
		dirMode = 0o700
	}
	rename := opts.Rename
	if rename == nil {
		rename = os.Rename
	}

	dir := filepath.Dir(path)
	_, statErr := os.Stat(dir)
	dirMissing := os.IsNotExist(statErr)
	if statErr != nil && !dirMissing {
		return statErr
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	if dirMissing || opts.ChmodExistingDir {
		if err := os.Chmod(dir, dirMode); err != nil {
			return err
		}
	}

	pattern := opts.TempPattern
	if pattern == "" {
		pattern = filepath.Base(path) + ".*.tmp"
	}
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := rename(tmpName, path); err != nil {
		return err
	}
	renamed = true

	if opts.SyncDir {
		if dirFile, err := os.Open(dir); err == nil {
			_ = dirFile.Sync()
			_ = dirFile.Close()
		}
	}
	return nil
}
