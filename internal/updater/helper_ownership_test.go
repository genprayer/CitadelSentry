package updater

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func helperCleanupFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "application")
	if err := os.WriteFile(target, []byte("old fixture application"), 0o700); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, ".citadel-sentry-helper-")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := copyHelper(target, directory)
	if err != nil {
		t.Fatal(err)
	}
	updated := []byte("new fixture application")
	digest := helperBytesHash(updated)
	if err := writeHelperOwnership(directory, target, helper, digest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, updated, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, readyFileName), []byte(digest), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory, target, helper
}

func TestRecordedHelperCleanupRemovesVerifiedFilesAndIsIdempotent(t *testing.T) {
	directory, target, _ := helperCleanupFixture(t)
	if err := cleanupRecordedHelper(directory, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("owned helper records remain: %v", err)
	}
	if err := cleanupRecordedHelper(directory, target); err != nil {
		t.Fatalf("repeated cleanup failed: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "new fixture application" {
		t.Fatal("cleanup changed the installed application")
	}
}

func TestRecordedHelperCleanupPreservesUnknownContents(t *testing.T) {
	for _, isDirectory := range []bool{false, true} {
		directory, target, helper := helperCleanupFixture(t)
		unknown := filepath.Join(directory, "player-notes")
		if isDirectory {
			if err := os.Mkdir(unknown, 0o700); err != nil {
				t.Fatal(err)
			}
			unknown = filepath.Join(unknown, "notes.txt")
		}
		if err := os.WriteFile(unknown, []byte("player data"), 0o600); err != nil {
			t.Fatal(err)
		}
		metadataPath := filepath.Join(directory, helperOwnershipFile)
		metadata, err := os.ReadFile(metadataPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := cleanupRecordedHelper(directory, target); err == nil {
			t.Fatal("unknown contents were ignored")
		}
		for _, path := range []string{helper, filepath.Join(directory, readyFileName), unknown} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("failed verification deleted %s: %v", path, err)
			}
		}
		after, err := os.ReadFile(metadataPath)
		if err != nil || !bytes.Equal(after, metadata) {
			t.Fatal("unknown directory was checkpointed as owned cleanup")
		}
	}
}

func TestRecordedHelperCleanupRejectsChangedFiles(t *testing.T) {
	for _, record := range []string{"helper", "ready", "target", "metadata", "missing-ready"} {
		t.Run(record, func(t *testing.T) {
			directory, target, helper := helperCleanupFixture(t)
			path := helper
			switch record {
			case "ready", "missing-ready":
				path = filepath.Join(directory, readyFileName)
			case "target":
				path = target
			case "metadata":
				path = filepath.Join(directory, helperOwnershipFile)
			}
			if record == "missing-ready" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("external change"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := cleanupRecordedHelper(directory, target); err == nil {
				t.Fatal("changed records allowed helper cleanup")
			}
			if _, err := os.Stat(helper); err != nil {
				t.Fatal("failed verification removed the helper")
			}
			if _, err := os.Stat(filepath.Join(directory, helperOwnershipFile)); err != nil {
				t.Fatal("failed verification removed ownership metadata")
			}
		})
	}
}

func TestRecordedHelperCleanupResumesInterruptedPayloadRemoval(t *testing.T) {
	directory, target, helper := helperCleanupFixture(t)
	record, err := readHelperOwnership(directory, target)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = helperPhaseCleaning
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, helperOwnershipFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(helper); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRecordedHelper(directory, target); err != nil {
		t.Fatalf("checkpointed cleanup could not resume: %v", err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("resumed cleanup left owned records")
	}
}

func TestHelperOwnershipCannotReplaceAnExistingRecord(t *testing.T) {
	directory, target, helper := helperCleanupFixture(t)
	path := filepath.Join(directory, helperOwnershipFile)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeHelperOwnership(directory, target, helper, strings.Repeat("a", 64)); err == nil {
		t.Fatal("existing ownership record was replaced")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("existing metadata changed")
	}
}

func TestHelperCleanupRejectsLinkedOwnershipAndHelperFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires native symlink privileges")
	}
	for _, isMetadata := range []bool{false, true} {
		directory, target, helper := helperCleanupFixture(t)
		path := helper
		if isMetadata {
			path = filepath.Join(directory, helperOwnershipFile)
		}
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		external := filepath.Join(filepath.Dir(directory), "external-record")
		if err := os.WriteFile(external, original, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, path); err != nil {
			t.Fatal(err)
		}
		if err := cleanupRecordedHelper(directory, target); err == nil {
			t.Fatal("linked records were treated as owned cleanup")
		}
		data, err := os.ReadFile(external)
		if err != nil || !bytes.Equal(data, original) {
			t.Fatal("linked external record changed")
		}
		if _, err := os.Lstat(path); err != nil {
			t.Fatal("linked record was removed")
		}
	}
}
