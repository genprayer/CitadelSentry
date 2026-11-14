package game

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryRestoresMissingRecordedSettingAcrossRestart(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	backupDir := filepath.Join(root, "backup")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\nCustomOption=Old\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, backupDir)
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[/Script/Squad.SquadGameUserSettings]\nCustomOption=New\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted := NewConfigManagerWithPaths(path, backupDir)
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("FrameRateLimit=144.000000")) || !bytes.Contains(data, []byte("CustomOption=New")) {
		t.Fatalf("missing original setting did not recover: %v, %s", err, data)
	}
	if status := restarted.GetStatus(); status.HasPendingRestore || status.ErrorMessage != "" || status.CurrentFpsCap != 144 {
		t.Fatalf("recovery was incomplete: %+v", status)
	}
}

func TestAutomaticRecoveryDistinguishesInterruptedFromFinishedSessionsAcrossRestart(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	backupDir := filepath.Join(root, "backup")
	if err := os.WriteFile(path, []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, backupDir)
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	restarted := NewConfigManagerWithPaths(path, backupDir)
	shouldRestore, err := restarted.ShouldRestoreAutomatically(false)
	if err != nil || !shouldRestore {
		t.Fatalf("interrupted session lost recovery on restart: restore=%t, error=%v", shouldRestore, err)
	}
	if err := restarted.MarkSessionFinished(); err != nil {
		t.Fatal(err)
	}
	restarted = NewConfigManagerWithPaths(path, backupDir)
	shouldRestore, err = restarted.ShouldRestoreAutomatically(false)
	if err != nil || shouldRestore {
		t.Fatalf("finished session ignored the user's opt-out: restore=%t, error=%v", shouldRestore, err)
	}
	shouldRestore, err = restarted.ShouldRestoreAutomatically(true)
	if err != nil || !shouldRestore {
		t.Fatalf("automatic restore preference was ignored: restore=%t, error=%v", shouldRestore, err)
	}
}

func TestAutomaticRecoveryReportsUnreadableManifest(t *testing.T) {
	root := t.TempDir()
	manager := NewConfigManagerWithPaths(filepath.Join(root, "GameUserSettings.ini"), filepath.Join(root, "backup"))
	if err := os.MkdirAll(manager.manifestPath(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ShouldRestoreAutomatically(false); err == nil {
		t.Fatal("unreadable recovery metadata was treated as an absent transaction")
	}
}
