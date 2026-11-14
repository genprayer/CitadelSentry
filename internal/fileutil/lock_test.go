package fileutil

import (
	"path/filepath"
	"testing"
)

func TestApplicationLockRejectsConcurrentOwnerAndReleasesIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.lock")
	release, err := TryLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	otherRelease, err := TryLockFile(path)
	if err == nil {
		otherRelease()
		t.Fatal("second application acquired the same state lock")
	}
	for range 2 {
		if err := release(); err != nil {
			t.Fatal(err)
		}
	}
	otherRelease, err = TryLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := otherRelease(); err != nil {
		t.Fatal(err)
	}
}
