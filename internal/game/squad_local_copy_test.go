package game

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestObservedSteamDiscoveryReadOnly(t *testing.T) {
	source := os.Getenv("CITADEL_TEST_SQUAD_CONFIG")
	if source == "" {
		t.Skip("set CITADEL_TEST_SQUAD_CONFIG for opt-in read-only discovery")
	}
	// The real path detector reads Steam metadata; backup state stays in a temp dir.
	manager, err := newConfigManager(t.TempDir(), detectSquadConfigPaths)
	if err != nil {
		t.Fatal(err)
	}
	status := manager.GetStatus()
	if !status.ConfigFound || status.ErrorMessage != "" || filepath.Clean(status.ConfigPath) != filepath.Clean(source) {
		t.Fatalf("actual Steam discovery failed: %+v", status)
	}
	t.Logf("Read-only discovery: ConfigFound=%t CurrentFpsCap=%g path=%s", status.ConfigFound, status.CurrentFpsCap, status.ConfigPath)
}

// TestObservedSquadConfigCopy exercises player-supplied bytes only in a temp copy.
func TestObservedSquadConfigCopy(t *testing.T) {
	source := os.Getenv("CITADEL_TEST_SQUAD_CONFIG")
	if source == "" {
		t.Skip("set CITADEL_TEST_SQUAD_CONFIG for the opt-in read-only source copy")
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	status := manager.GetStatus()
	if !status.ConfigFound || status.ErrorMessage != "" {
		t.Fatalf("observed source was not recognized: %+v", status)
	}
	t.Logf("Recognized copied source: original FPS=%g", status.CurrentFpsCap)
	if err := manager.ApplyPotatoConfig(15, true); err != nil {
		t.Fatal(err)
	}
	if status := manager.GetStatus(); status.ErrorMessage != "" || status.CurrentFpsCap != 15 {
		t.Fatalf("copied source patch failed: %+v", status)
	}
	restarted := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("copied source was not restored exactly: %v", err)
	}
}
