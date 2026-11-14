//go:build linux

package platform

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

func TestThreadResourceAcquiresAndReleasesOnItsOriginalThread(t *testing.T) {
	var acquiredThread, releasedThread int
	var releases atomic.Int64
	release, err := acquireThreadResource(func() error {
		acquiredThread = syscall.Gettid()
		return nil
	}, func() error {
		releasedThread = syscall.Gettid()
		releases.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var callers sync.WaitGroup
	for range 8 {
		callers.Add(1)
		go func() { defer callers.Done(); release() }()
	}
	callers.Wait()
	if acquiredThread != releasedThread || releases.Load() != 1 {
		t.Fatalf("resource was cleared on another thread or more than once: acquire=%d release=%d count=%d", acquiredThread, releasedThread, releases.Load())
	}
}

func TestThreadResourceAcquisitionErrorLeavesNoReleaseTask(t *testing.T) {
	failure := errors.New("resource unavailable")
	var releases atomic.Int64
	release, err := acquireThreadResource(func() error { return failure }, func() error { releases.Add(1); return nil })
	if !errors.Is(err, failure) || release != nil || releases.Load() != 0 {
		t.Fatalf("failed acquisition was reported as a live resource: %v", err)
	}
}
