//go:build !windows

package fileutil

import (
	"fmt"
	"os"
	"syscall"
)

func fileLinkCount(_ *os.File, info os.FileInfo) (uint64, error) {
	stat, isSupported := info.Sys().(*syscall.Stat_t)
	if !isSupported {
		return 0, fmt.Errorf("file metadata does not expose a link count")
	}
	return uint64(stat.Nlink), nil
}
