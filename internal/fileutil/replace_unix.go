//go:build !windows

package fileutil

import (
	"os"
	"path/filepath"
)

func replaceFile(source string, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(destination))
}

func createFile(source string, destination string) error {
	// Linking publishes the complete temporary file without replacing a new file.
	if err := os.Link(source, destination); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(destination))
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer directory.Close()
	return directory.Sync()
}

// SetReadOnly sets or removes read-only permissions on a file.
func SetReadOnly(path string, readOnly bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if readOnly {
		mode &^= 0o222
	} else {
		mode |= 0o200
	}
	return os.Chmod(path, mode)
}
