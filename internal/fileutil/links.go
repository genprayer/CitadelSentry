package fileutil

import (
	"errors"
	"fmt"
	"os"
)

// VerifySingleFileLink rejects replacement that would sever a shared file's links.
func VerifySingleFileLink(path string, expected os.FileInfo) (failure error) {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file to inspect links: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			failure = errors.Join(failure, fmt.Errorf("close inspected file: %w", err))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect file identity: %w", err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(expected, info) {
		return fmt.Errorf("file changed while inspecting links: %s", path)
	}
	count, err := fileLinkCount(file, info)
	if err != nil {
		return fmt.Errorf("inspect file link count: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("refusing replacement of a shared or unverifiable file: %s (links=%d)", path, count)
	}
	return nil
}
