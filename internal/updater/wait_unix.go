//go:build !windows

package updater

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

func waitForParent(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect old process: %w", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("old application did not exit before update timeout")
}
