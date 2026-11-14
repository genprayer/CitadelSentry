package fileutil

import (
	"fmt"
	"os"
	"sync"
)

// TryLockFile holds an exclusive OS lock until release or process exit.
// The stable lock file must not be removed while another process may use it.
func TryLockFile(path string) (func() error, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open application lock: %w", err)
	}
	if err := lockFile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("acquire application lock: %w", err)
	}
	var once sync.Once
	var releaseError error
	return func() error {
		once.Do(func() { releaseError = file.Close() })
		return releaseError
	}, nil
}
