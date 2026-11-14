package game

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStatusRejectsCorruptOrMissingSessionRecords(t *testing.T) {
	for _, test := range []struct {
		name                            string
		isPrimary, isPatched, isMissing bool
	}{
		{name: "primary-original", isPrimary: true},
		{name: "secondary-original"},
		{name: "secondary-patch", isPatched: true},
		{name: "missing-secondary", isMissing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "GameUserSettings.ini")
			if err := os.WriteFile(path, []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "Engine.ini"), []byte("[SystemSettings]\nt.MaxFPS=120\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
			if err := manager.ApplyPotatoConfig(1, false); err != nil {
				t.Fatal(err)
			}
			manifest, err := manager.readManifestLocked()
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range manifest.Files {
				if (entry.Path == path) != test.isPrimary {
					continue
				}
				name := entry.BackupName
				if test.isPatched {
					name = entry.PatchedName
				}
				record := filepath.Join(manager.backupDir, name)
				if test.isMissing {
					err = os.Remove(record)
				} else {
					err = os.WriteFile(record, []byte("corrupt"), 0o600)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			status := manager.GetStatus()
			if status.ErrorMessage == "" || status.HasBackup {
				t.Fatalf("unverified records were shown as a working backup: %+v", status)
			}
			if !status.HasPendingRestore {
				t.Fatal("corruption discarded the recovery obligation")
			}
		})
	}
}

func TestCorruptBaselineCannotReportSuccessfulBackupOrReplaceRecords(t *testing.T) {
	for _, isForced := range []bool{false, true} {
		root := t.TempDir()
		path := filepath.Join(root, "GameUserSettings.ini")
		if err := os.WriteFile(path, []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
		if err := manager.BackupConfig(); err != nil {
			t.Fatal(err)
		}
		metadata, err := os.ReadFile(manager.manifestPath())
		if err != nil {
			t.Fatal(err)
		}
		corrupt := []byte("corrupt")
		if err := os.WriteFile(manager.BackupPath(), corrupt, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := manager.BackupConfig(); err == nil {
			t.Fatal("corrupt baseline was reported as a successful backup")
		}
		if err := manager.SaveCurrentAsBackup(isForced); err == nil {
			t.Fatal("unknown original was replaced")
		}
		data, err := os.ReadFile(manager.BackupPath())
		if err != nil || !bytes.Equal(data, corrupt) {
			t.Fatalf("backup evidence changed: %v", err)
		}
		data, err = os.ReadFile(manager.manifestPath())
		if err != nil || !bytes.Equal(data, metadata) {
			t.Fatalf("metadata changed: %v", err)
		}
	}
}

func TestManifestSymlinkIsRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires native symlink privileges")
	}
	root := t.TempDir()
	manager := NewConfigManagerWithPaths(filepath.Join(root, "GameUserSettings.ini"), filepath.Join(root, "backup"))
	if err := os.Mkdir(manager.backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "external-manifest.json")
	if err := os.WriteFile(target, []byte(`{"originalPath":"/unused"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, manager.manifestPath()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.readManifestLocked(); err == nil {
		t.Fatal("linked metadata was accepted")
	}
	if !manager.HasActiveBackup() || manager.GetStatus().ErrorMessage == "" {
		t.Fatal("unknown metadata was presented as normal")
	}
}

func TestStatusReportsUnreadableSettings(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires enforced Unix read permissions")
	}
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	if err := os.WriteFile(path, []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Errorf("restore fixture permissions: %v", err)
		}
	})
	if status := manager.GetStatus(); status.ErrorMessage == "" {
		t.Fatal("unreadable configuration was presented as normal")
	}
}

func TestStatusDoesNotReadFpsFromCommentsOrUnrelatedSections(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	data := []byte("; FrameRateLimit=30.000000\n[Other]\nFrameRateLimit=60.000000\n[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.500000\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	status := NewConfigManagerWithPaths(path, filepath.Join(root, "backup")).GetStatus()
	if status.ErrorMessage != "" || float64(status.CurrentFpsCap) != 144.5 {
		t.Fatalf("diagnostic invented the effective FPS: %+v", status)
	}
}

func TestStatusReportsUnknownOrAmbiguousFps(t *testing.T) {
	for _, data := range []string{
		"[/Script/Squad.SquadGameUserSettings]\nOther=Keep\n",
		"[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=NaN\n",
		"[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=30\nFrameRateLimit=144\n",
		"[/Script/Engine.GameUserSettings]\nFrameRateLimit=30\n[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144\n",
	} {
		root := t.TempDir()
		path := filepath.Join(root, "GameUserSettings.ini")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		status := NewConfigManagerWithPaths(path, filepath.Join(root, "backup")).GetStatus()
		if status.ErrorMessage == "" {
			t.Fatalf("unknown FPS was displayed as a known value: %+v", status)
		}
	}
}

func TestCommentedPotatoMarkerDoesNotBlockAnOriginalBackup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	data := []byte("; PotatoMode=True\n[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if manager.GetStatus().IsPotatoMode {
		t.Fatal("a comment was treated as an applied game configuration")
	}
	if err := manager.BackupConfig(); err != nil {
		t.Fatalf("a comment blocked backing up real original settings: %v", err)
	}
}
