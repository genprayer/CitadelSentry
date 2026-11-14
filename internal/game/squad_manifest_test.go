package game

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestInvalidSquadManifestCannotReplaceVerifiedOriginals(t *testing.T) {
	for _, test := range []struct {
		name    string
		corrupt func([]byte) []byte
	}{
		{name: "null manifest", corrupt: func([]byte) []byte { return []byte("null") }},
		{name: "null active flag", corrupt: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"active": true`), []byte(`"active": null`), 1)
		}},
		{name: "missing active flag", corrupt: func(data []byte) []byte { return bytes.Replace(data, []byte(`"active": true,`), nil, 1) }},
		{name: "duplicate active flag", corrupt: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"active": true`), []byte(`"active": true, "active": false`), 1)
		}},
		{name: "missing transaction version", corrupt: func(data []byte) []byte { return bytes.Replace(data, []byte(`"version": 2,`), nil, 1) }},
		{name: "noncanonical file records key", corrupt: func(data []byte) []byte { return bytes.Replace(data, []byte(`"files":`), []byte(`"Files":`), 1) }},
		{name: "null original permissions", corrupt: func(data []byte) []byte {
			return regexp.MustCompile(`"originalMode": \d+`).ReplaceAll(data, []byte(`"originalMode": null`))
		}},
		{name: "invalid original permissions", corrupt: func(data []byte) []byte {
			return regexp.MustCompile(`"originalMode": \d+`).ReplaceAll(data, []byte(`"originalMode": 65535`))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "GameUserSettings.ini")
			original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
			if err := manager.ApplyPotatoConfig(30, false); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(manager.manifestPath())
			if err != nil {
				t.Fatal(err)
			}
			corrupt := test.corrupt(data)
			if bytes.Equal(data, corrupt) {
				t.Fatal("fixture did not corrupt metadata")
			}
			if err := os.WriteFile(manager.manifestPath(), corrupt, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := manager.SaveCurrentAsBackup(false); err == nil {
				t.Fatal("unknown active state replaced the original with temporary settings")
			}
			backup, err := os.ReadFile(manager.BackupPath())
			if err != nil || !bytes.Equal(backup, original) {
				t.Fatalf("verified original was lost: %v", err)
			}
			current, err := os.ReadFile(manager.manifestPath())
			if err != nil || !bytes.Equal(current, corrupt) {
				t.Fatalf("invalid metadata was replaced: %v", err)
			}
			if !manager.HasActiveBackup() || manager.GetStatus().ErrorMessage == "" {
				t.Fatal("unknown recovery state was presented as normal")
			}
			if err := manager.ApplyPotatoConfig(5, false); err == nil {
				t.Fatal("invalid recovery state allowed a new patch")
			}
			if err := manager.RestoreOriginalConfig(); err == nil {
				t.Fatal("invalid recovery state reported successful restoration")
			}
			if err := os.WriteFile(manager.manifestPath(), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := manager.RestoreOriginalConfig(); err != nil {
				t.Fatal(err)
			}
			restored, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(restored, original) || manager.HasActiveBackup() {
				t.Fatalf("verified metadata did not remain recoverable: %v", err)
			}
		})
	}
}

func TestMissingManifestCannotReplaceOrphanedOriginals(t *testing.T) {
	for _, isForced := range []bool{false, true} {
		root := t.TempDir()
		path := filepath.Join(root, "GameUserSettings.ini")
		original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
		if err := manager.ApplyPotatoConfig(30, false); err != nil {
			t.Fatal(err)
		}
		metadata, err := os.ReadFile(manager.manifestPath())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(manager.manifestPath()); err != nil {
			t.Fatal(err)
		}
		manager = NewConfigManagerWithPaths(path, manager.backupDir)
		if err := manager.SaveCurrentAsBackup(isForced); err == nil {
			t.Fatal("missing metadata replaced the original with temporary settings")
		}
		backup, err := os.ReadFile(manager.BackupPath())
		if err != nil || !bytes.Equal(backup, original) {
			t.Fatalf("orphaned original was lost: %v", err)
		}
		if !manager.HasActiveBackup() || manager.GetStatus().ErrorMessage == "" {
			t.Fatal("unknown recovery state was presented as normal")
		}
		if err := os.WriteFile(manager.manifestPath(), metadata, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := manager.RestoreOriginalConfig(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, original) {
			t.Fatalf("retained originals were not recoverable: %v", err)
		}
	}
}

func TestDirectoryAliasesDoNotPatchTheSamePhysicalConfigsTwice(t *testing.T) {
	root := t.TempDir()
	realDirectory := filepath.Join(root, "real")
	aliasDirectory := filepath.Join(root, "alias")
	if err := os.MkdirAll(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDirectory, aliasDirectory); err != nil {
		t.Skipf("directory alias fixture is unavailable: %v", err)
	}
	path := filepath.Join(realDirectory, "GameUserSettings.ini")
	enginePath := filepath.Join(realDirectory, "Engine.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	engineOriginal := []byte("[SystemSettings]\nt.MaxFPS=144.000000\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enginePath, engineOriginal, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	manager.secondaryPaths = []string{filepath.Join(aliasDirectory, "GameUserSettings.ini")}
	if err := manager.ApplyPotatoConfig(30, false); err != nil {
		t.Fatalf("one physical config was treated as competing changes: %v", err)
	}
	manifest, err := manager.readManifestLocked()
	if err != nil || len(manifest.Files) != 2 {
		t.Fatalf("physical configs were duplicated: records=%d error=%v", len(manifest.Files), err)
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	for _, record := range []struct {
		path string
		data []byte
	}{{path, original}, {enginePath, engineOriginal}, {filepath.Join(aliasDirectory, "GameUserSettings.ini"), original}} {
		data, err := os.ReadFile(record.path)
		if err != nil || !bytes.Equal(data, record.data) {
			t.Fatalf("alias recovery lost originals: %v", err)
		}
	}
}

func TestSymlinkedConfigFileCannotLoseItsOriginalNodeDuringPatch(t *testing.T) {
	root := t.TempDir()
	realPath := filepath.Join(root, "original-settings.ini")
	path := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	if err := os.WriteFile(realPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, path); err != nil {
		t.Skipf("file link fixture is unavailable: %v", err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(30, false); err == nil {
		t.Fatal("atomic replacement destroyed a configuration link")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("original configuration link was replaced: %v", err)
	}
	data, err := os.ReadFile(realPath)
	if err != nil || !bytes.Equal(data, original) || manager.HasActiveBackup() {
		t.Fatalf("rejected file link changed the original: %v", err)
	}
}

func TestRecoveryRejectsAReplacedConfigLinkBeforeRestoringOtherFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	enginePath := filepath.Join(root, "Engine.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enginePath, []byte("[SystemSettings]\nt.MaxFPS=144.000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(30, false); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(root, "replacement-engine.ini")
	if err := os.Rename(enginePath, targetPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, enginePath); err != nil {
		t.Skipf("file link fixture is unavailable: %v", err)
	}
	if err := manager.RestoreOriginalConfig(); err == nil {
		t.Fatal("recovery replaced a competing file link")
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, patched) || !manager.HasActiveBackup() {
		t.Fatalf("recovery mutated another file before validating all targets: %v", err)
	}
	info, err := os.Lstat(enginePath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("competing link was lost: %v", err)
	}
}

func TestHardLinkedConfigCannotLoseItsOriginalFileRelationship(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	otherPath := filepath.Join(root, "shared-settings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, otherPath); err != nil {
		t.Skipf("hard link fixture is unavailable: %v", err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(30, false); err == nil {
		t.Fatal("atomic replacement broke a configuration hard link")
	}
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(otherPath)
	if err != nil || !os.SameFile(first, second) {
		t.Fatalf("original file relationship was lost: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) || manager.HasActiveBackup() {
		t.Fatalf("rejected hard link changed original data: %v", err)
	}
}
