package game

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImmutableBaselineCapturedAndRestored(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "GameUserSettings.ini")
	backupDir := filepath.Join(root, "squad_backup")

	initialContent := []byte("[/Script/Squad.SQGameUserSettings]\r\nFrameRateLimit=120.000000\r\nCustomSetting=OriginalBaseline\r\n")
	if err := os.WriteFile(configPath, initialContent, 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := NewConfigManagerWithPaths(configPath, backupDir)
	if !mgr.HasInitialBaseline() {
		t.Fatal("expected baseline to be captured upon initialization")
	}

	// Verify baseline.json exists
	baselineMeta := filepath.Join(backupDir, "baseline", "baseline.json")
	if _, err := os.Stat(baselineMeta); err != nil {
		t.Fatalf("expected baseline.json at %s, got: %v", baselineMeta, err)
	}

	// Change file to something else (e.g., user changed settings)
	modifiedContent := []byte("[/Script/Squad.SQGameUserSettings]\r\nFrameRateLimit=60.000000\r\nCustomSetting=UserModified\r\n")
	if err := os.WriteFile(configPath, modifiedContent, 0o600); err != nil {
		t.Fatal(err)
	}

	// Running EnsureInitialBaselineCaptured or SaveCurrentAsBackup must NEVER overwrite the baseline
	if err := mgr.EnsureInitialBaselineCaptured(); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveCurrentAsBackup(true); err != nil {
		t.Fatal(err)
	}

	// Verify baseline file still has the original initialContent
	savedBaselineFile := filepath.Join(backupDir, "baseline", "GameUserSettings.ini")
	savedContent, err := os.ReadFile(savedBaselineFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(savedContent, initialContent) {
		t.Fatalf("immutable baseline was overwritten! got: %s, want: %s", savedContent, initialContent)
	}

	// Now perform 1-click restore to initial baseline
	if err := mgr.RestoreInitialBaseline(); err != nil {
		t.Fatalf("RestoreInitialBaseline failed: %v", err)
	}

	restoredContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restoredContent, initialContent) {
		t.Fatalf("restored content does not match initial baseline! got: %s, want: %s", restoredContent, initialContent)
	}
}

func TestMultiRestorePoints(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "GameUserSettings.ini")
	backupDir := filepath.Join(root, "squad_backup")

	state1 := []byte("[/Script/Squad.SQGameUserSettings]\r\nFrameRateLimit=144.000000\r\nState=State1\r\n")
	if err := os.WriteFile(configPath, state1, 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := NewConfigManagerWithPaths(configPath, backupDir)

	// Create restore point 1
	rp1, err := mgr.CreateRestorePoint("Point 1 - Ultra High", "Before lowering settings")
	if err != nil {
		t.Fatalf("CreateRestorePoint 1 failed: %v", err)
	}
	if rp1.Name != "Point 1 - Ultra High" || rp1.ID == "" {
		t.Fatalf("unexpected restore point 1: %+v", rp1)
	}

	time.Sleep(10 * time.Millisecond)

	// Change state to state 2
	state2 := []byte("[/Script/Squad.SQGameUserSettings]\r\nFrameRateLimit=75.000000\r\nState=State2\r\n")
	if err := os.WriteFile(configPath, state2, 0o600); err != nil {
		t.Fatal(err)
	}

	// Create restore point 2
	rp2, err := mgr.CreateRestorePoint("Point 2 - Medium", "Balanced settings")
	if err != nil {
		t.Fatalf("CreateRestorePoint 2 failed: %v", err)
	}

	// List restore points: should have 2 points, sorted newest first (rp2 then rp1)
	points, err := mgr.ListRestorePoints()
	if err != nil {
		t.Fatalf("ListRestorePoints failed: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 restore points, got %d", len(points))
	}
	if points[0].ID != rp2.ID || points[1].ID != rp1.ID {
		t.Fatalf("expected order [rp2, rp1], got [%s, %s]", points[0].ID, points[1].ID)
	}

	// Restore Point 1
	if err := mgr.RestoreRestorePoint(rp1.ID); err != nil {
		t.Fatalf("RestoreRestorePoint 1 failed: %v", err)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "State=State1") {
		t.Fatalf("content was not restored to State1: %s", content)
	}

	// Restore Point 2
	if err := mgr.RestoreRestorePoint(rp2.ID); err != nil {
		t.Fatalf("RestoreRestorePoint 2 failed: %v", err)
	}
	content, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "State=State2") {
		t.Fatalf("content was not restored to State2: %s", content)
	}

	// Delete Point 1
	if err := mgr.DeleteRestorePoint(rp1.ID); err != nil {
		t.Fatalf("DeleteRestorePoint failed: %v", err)
	}
	pointsAfterDelete, err := mgr.ListRestorePoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(pointsAfterDelete) != 1 || pointsAfterDelete[0].ID != rp2.ID {
		t.Fatalf("expected 1 restore point (rp2), got: %+v", pointsAfterDelete)
	}

	// Test security: invalid or traversal IDs must be strictly rejected
	for _, invalidID := range []string{"", ".", "..", "/", "\\", "../etc", "rp-../../something", "custom_id"} {
		if err := mgr.DeleteRestorePoint(invalidID); err == nil {
			t.Errorf("DeleteRestorePoint(%q) should have been rejected", invalidID)
		}
		if err := mgr.RestoreRestorePoint(invalidID); err == nil {
			t.Errorf("RestoreRestorePoint(%q) should have been rejected", invalidID)
		}
	}
}
