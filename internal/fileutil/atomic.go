package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces a file only after its complete contents reach disk.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	return writeFileAtomic(path, data, mode, replaceFile)
}

// CreateFileAtomic publishes a complete file only if the destination is absent.
func CreateFileAtomic(path string, data []byte, mode os.FileMode) error {
	return writeFileAtomic(path, data, mode, createFile)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode, publish func(string, string) error) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".citadel-sentry-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := publish(temporaryPath, path); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}
	return nil
}
