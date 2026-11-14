package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFileAtomicPreservesConcurrentWinner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "Engine.ini")
	start := make(chan struct{})
	type result struct {
		data string
		err  error
	}
	results := make(chan result, 2)
	for _, data := range []string{"verified backup", "new game settings"} {
		go func() {
			<-start
			results <- result{data: data, err: CreateFileAtomic(path, []byte(data), 0o644)}
		}()
	}
	close(start)
	winner := ""
	for range 2 {
		result := <-results
		if result.err == nil {
			if winner != "" {
				t.Fatal("both creators replaced the destination")
			}
			winner = result.data
		} else if !errors.Is(result.err, os.ErrExist) {
			t.Fatalf("unexpected creation error: %v", result.err)
		}
	}
	current, err := os.ReadFile(path)
	if err != nil || winner == "" || string(current) != winner {
		t.Fatalf("concurrent winner was overwritten: %q, winner %q, error %v", current, winner, err)
	}
	if err := CreateFileAtomic(path, []byte("stale backup"), 0o644); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing file was accepted: %v", err)
	}
	current, err = os.ReadFile(path)
	if err != nil || string(current) != winner {
		t.Fatalf("existing settings were changed: %q, error %v", current, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "Engine.ini" {
		t.Fatalf("temporary files were not cleaned up: %v, error %v", entries, err)
	}
}

func TestWriteFileAtomicCreatesAndReplacesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	if err := WriteFileAtomic(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic(first) error = %v", err)
	}
	if err := WriteFileAtomic(path, []byte("second"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic(second) error = %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) != "second" {
		t.Fatalf("contents = %q, want second", contents)
	}
}

func TestSetReadOnlyAndReplace(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "readonly.txt")
	if err := WriteFileAtomic(path, []byte("initial"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic error = %v", err)
	}

	if err := SetReadOnly(path, true); err != nil {
		t.Fatalf("SetReadOnly(true) error = %v", err)
	}

	// Writing atomic over a read-only file should succeed because replaceFile clears read-only
	if err := WriteFileAtomic(path, []byte("overwritten"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic over read-only failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(data) != "overwritten" {
		t.Fatalf("expected overwritten, got %q", string(data))
	}

	if err := SetReadOnly(path, false); err != nil {
		t.Fatalf("SetReadOnly(false) error = %v", err)
	}
}

func TestFailedReplacementPreservesOriginalPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.ini")
	if err := os.WriteFile(path, []byte("original"), 0o400); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(filepath.Join(root, "missing-source"), path); err == nil {
		t.Fatal("missing source was reported as replaced")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode().Perm() != after.Mode().Perm() {
		t.Fatal("failed replacement changed original permissions")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatalf("failed replacement lost original bytes: %v", err)
	}
}
