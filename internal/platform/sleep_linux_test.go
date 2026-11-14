//go:build linux

package platform

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

func testInhibitorDescriptor(t *testing.T) dbus.UnixFD {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	duplicate, err := unix.Dup(int(reader.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	var original unix.Stat_t
	if err := unix.Fstat(duplicate, &original); err != nil {
		unix.Close(duplicate)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var current unix.Stat_t
		if err := unix.Fstat(duplicate, &current); err == nil && current.Dev == original.Dev && current.Ino == original.Ino {
			unix.Close(duplicate)
		}
	})
	return dbus.UnixFD(duplicate)
}

func assertInhibitorClosed(t *testing.T, descriptor dbus.UnixFD) {
	t.Helper()
	if _, err := unix.FcntlInt(uintptr(descriptor), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
		t.Fatalf("inhibitor descriptor is still owned: %v", err)
	}
}

func TestLinuxSleepInhibitorRequiresAnActualDescriptor(t *testing.T) {
	for _, reply := range [][]any{nil, {true}, {dbus.UnixFD(-1)}} {
		release, err := acquireLinuxSleepInhibitor(context.Background(), "test", func(context.Context, string) ([]any, error) { return reply, nil })
		if err == nil || release != nil {
			t.Fatal("unacknowledged inhibition reported success")
		}
	}
}

func TestLinuxSleepInhibitorReleaseOwnsOneNonInheritedDescriptor(t *testing.T) {
	descriptor := testInhibitorDescriptor(t)
	release, err := acquireLinuxSleepInhibitor(context.Background(), "test", func(context.Context, string) ([]any, error) { return []any{descriptor}, nil })
	if err != nil {
		t.Fatal(err)
	}
	flags, err := unix.FcntlInt(uintptr(descriptor), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("inhibitor leaked to child processes: %v", err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() { defer workers.Done(); release() }()
	}
	workers.Wait()
	assertInhibitorClosed(t, descriptor)
}

func TestLinuxSleepInhibitorClosesReceivedDescriptorsOnFailureAndCancellation(t *testing.T) {
	for _, scenario := range []string{"error", "malformed reply", "cancelled reply"} {
		t.Run(scenario, func(t *testing.T) {
			descriptor := testInhibitorDescriptor(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			release, err := acquireLinuxSleepInhibitor(ctx, "test", func(context.Context, string) ([]any, error) {
				switch scenario {
				case "error":
					return []any{descriptor}, errors.New("request failed")
				case "malformed reply":
					return []any{descriptor, "unexpected"}, nil
				default:
					cancel()
					return []any{descriptor}, nil
				}
			})
			if err == nil || release != nil {
				t.Fatal("failed acquisition reported success")
			}
			assertInhibitorClosed(t, descriptor)
		})
	}
}

func TestLinuxSleepInhibitorDoesNotRequestAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hasRequested := false
	_, err := acquireLinuxSleepInhibitor(ctx, "test", func(context.Context, string) ([]any, error) { hasRequested = true; return nil, nil })
	if !errors.Is(err, context.Canceled) || hasRequested {
		t.Fatal("cancelled session acquired a new system resource")
	}
}
