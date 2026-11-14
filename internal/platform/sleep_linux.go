//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"citadel-seeder/internal/logger"
	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

const sleepInhibitorRequestTimeout = 8 * time.Second

type sleepInhibitorRequest func(context.Context, string) ([]any, error)

func requestLoginSleepInhibitor(ctx context.Context, reason string) (reply []any, result error) {
	connection, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("connect sleep inhibitor bus: %w", err)
	}
	defer func() { result = errors.Join(result, connection.Close()) }()
	if !connection.SupportsUnixFDs() {
		return nil, fmt.Errorf("system bus does not support inhibitor file descriptors")
	}
	call := connection.Object("org.freedesktop.login1", "/org/freedesktop/login1").CallWithContext(
		ctx, "org.freedesktop.login1.Manager.Inhibit", 0, "idle:sleep", "Citadel Sentry", reason, "block",
	)
	return call.Body, call.Err
}

// PreventSleep owns the actual logind inhibitor descriptor, with no helper processes.
func (a *linuxAdapter) PreventSleep(ctx context.Context, reason string) (ReleaseFunc, error) {
	return acquireLinuxSleepInhibitor(ctx, reason, requestLoginSleepInhibitor)
}

func acquireLinuxSleepInhibitor(parent context.Context, reason string, request sleepInhibitorRequest) (ReleaseFunc, error) {
	if err := parent.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, sleepInhibitorRequestTimeout)
	defer cancel()
	// Receiving an FD and setting CLOEXEC must not race Go child-process creation.
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reply, err := request(ctx, reason)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && len(reply) != 1 {
		err = fmt.Errorf("sleep inhibitor returned an unexpected response")
	}
	var descriptor dbus.UnixFD
	if err == nil {
		var isDescriptor bool
		descriptor, isDescriptor = reply[0].(dbus.UnixFD)
		if !isDescriptor || descriptor < 0 {
			err = fmt.Errorf("sleep inhibitor returned no valid descriptor")
		}
	}
	if err != nil {
		return nil, errors.Join(err, closeInhibitorReplyDescriptors(reply))
	}
	flags, err := unix.FcntlInt(uintptr(descriptor), unix.F_GETFD, 0)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("verify sleep inhibitor descriptor: %w", err), unix.Close(int(descriptor)))
	}
	if _, err := unix.FcntlInt(uintptr(descriptor), unix.F_SETFD, flags|unix.FD_CLOEXEC); err != nil {
		return nil, errors.Join(fmt.Errorf("protect sleep inhibitor from child inheritance: %w", err), unix.Close(int(descriptor)))
	}
	file := os.NewFile(uintptr(descriptor), "Citadel Sentry sleep inhibitor")
	if file == nil {
		return nil, errors.Join(fmt.Errorf("sleep inhibitor descriptor is unavailable"), unix.Close(int(descriptor)))
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if err := file.Close(); err != nil {
				logger.Error("Could not release sleep inhibitor descriptor", "error", err)
			}
		})
	}, nil
}

func closeInhibitorReplyDescriptors(reply []any) error {
	seen := make(map[dbus.UnixFD]bool)
	var failures []error
	for _, value := range reply {
		if descriptor, isDescriptor := value.(dbus.UnixFD); isDescriptor && descriptor >= 0 && !seen[descriptor] {
			seen[descriptor] = true
			if err := unix.Close(int(descriptor)); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
