//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type loginInhibitor struct {
	What      string
	Who       string
	Why       string
	Mode      string
	UserID    uint32
	ProcessID uint32
}

// This opt-in test acquires and releases one owned lock; it performs no power action.
func TestNativeLogindInhibitorLifecycle(t *testing.T) {
	if os.Getenv("CITADEL_TEST_LOGIND") != "1" {
		t.Skip("local logind acceptance requires explicit test selection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	connection, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	reason := fmt.Sprintf("Citadel Sentry owned acceptance test PID %d", os.Getpid())
	countOwned := func() (int, error) {
		var entries []loginInhibitor
		err := connection.Object("org.freedesktop.login1", "/org/freedesktop/login1").CallWithContext(ctx, "org.freedesktop.login1.Manager.ListInhibitors", 0).Store(&entries)
		if err != nil {
			return 0, err
		}
		count := 0
		for _, entry := range entries {
			if entry.ProcessID == uint32(os.Getpid()) && entry.Why == reason {
				kinds := strings.Split(entry.What, ":")
				hasRequiredKinds := len(kinds) == 2 && ((kinds[0] == "idle" && kinds[1] == "sleep") || (kinds[0] == "sleep" && kinds[1] == "idle"))
				if entry.Who != "Citadel Sentry" || entry.Mode != "block" || !hasRequiredKinds {
					return 0, fmt.Errorf("owned inhibitor has unexpected metadata: what=%q who=%q mode=%q", entry.What, entry.Who, entry.Mode)
				}
				count++
			}
		}
		return count, nil
	}
	if count, err := countOwned(); err != nil || count != 0 {
		t.Fatalf("unexpected preexisting owned inhibitor: count=%d error=%v", count, err)
	}
	release, err := NewAdapter().PreventSleep(ctx, reason)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if count, err := countOwned(); err != nil || count != 1 {
		t.Fatalf("actual inhibitor was not acknowledged: count=%d error=%v", count, err)
	}
	release()
	deadline := time.Now().Add(time.Second)
	for {
		count, err := countOwned()
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("owned inhibitor remained after release")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Log("One actual local logind inhibitor acquired and released; owned count returned to zero")
}
