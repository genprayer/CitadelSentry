package game

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSquadConfigManager_PotatoModeAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "Config", "WindowsNoEditor")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir configDir: %v", err)
	}

	gameUserSettingsPath := filepath.Join(configDir, "GameUserSettings.ini")
	inputIniPath := filepath.Join(configDir, "Input.ini")
	backupDir := filepath.Join(tempDir, "citadel-sentry", "squad_backup")

	// 1. Create original GameUserSettings.ini with realistic player settings
	originalUserSettings := []byte(`[/Script/Squad.SquadGameUserSettings]
ResolutionSizeX=2560
ResolutionSizeY=1440
LastUserConfirmedResolutionSizeX=2560
LastUserConfirmedResolutionSizeY=1440
WindowMode=0
FullscreenMode=0
FrameRateLimit=144.000000
bUseVSync=True
CustomUserOption=PreserveMePlease

[ScalabilityGroups]
sg.ResolutionQuality=100.000000
sg.ViewDistanceQuality=3
sg.AntiAliasingQuality=3
sg.ShadowQuality=3
sg.PostProcessQuality=3
sg.TextureQuality=3
sg.EffectsQuality=3
sg.FoliageQuality=3
sg.ShadingQuality=3
`)
	if err := os.WriteFile(gameUserSettingsPath, originalUserSettings, 0o644); err != nil {
		t.Fatalf("write GameUserSettings.ini: %v", err)
	}

	// 2. Create Input.ini (Keybindings & Controls)
	originalInputContent := []byte(`[/Script/Engine.InputSettings]
ActionMappings=(ActionName="Fire",bShift=False,bCtrl=False,bAlt=False,bCmd=False,Key=LeftMouseButton)
ActionMappings=(ActionName="Jump",bShift=False,bCtrl=False,bAlt=False,bCmd=False,Key=SpaceBar)
AxisMappings=(AxisName="Turn",Scale=1.000000,Key=MouseX)
AxisMappings=(AxisName="LookUp",Scale=-1.000000,Key=MouseY)
MouseSensitivity=0.350000
`)
	if err := os.WriteFile(inputIniPath, originalInputContent, 0o644); err != nil {
		t.Fatalf("write Input.ini: %v", err)
	}
	originalInputHash := sha256.Sum256(originalInputContent)

	mgr := NewConfigManagerWithPaths(gameUserSettingsPath, backupDir)

	// Verify initial status
	status := mgr.GetStatus()
	if !status.ConfigFound {
		t.Fatalf("expected config to be found")
	}
	if status.HasBackup {
		t.Fatalf("expected no backup initially")
	}
	if status.IsPotatoMode {
		t.Fatalf("expected not potato mode initially")
	}
	if status.CurrentFpsCap != 144 {
		t.Fatalf("expected initial FPS cap 144, got %g", status.CurrentFpsCap)
	}

	// 3. Apply Potato Mode (1 FPS, lowResolution=true)
	if err := mgr.ApplyPotatoConfig(1, true); err != nil {
		t.Fatalf("ApplyPotatoConfig failed: %v", err)
	}

	// Check that backup was created
	if !mgr.HasActiveBackup() {
		t.Fatalf("expected active backup after applying potato mode")
	}

	// Check status now
	status = mgr.GetStatus()
	if !status.HasBackup {
		t.Fatalf("status reports HasBackup=false")
	}
	if !status.IsPotatoMode {
		t.Fatalf("status reports IsPotatoMode=false")
	}
	if status.CurrentFpsCap != 1 {
		t.Fatalf("expected FPS cap 1, got %g", status.CurrentFpsCap)
	}

	// Check GameUserSettings.ini content
	modifiedData, err := os.ReadFile(gameUserSettingsPath)
	if err != nil {
		t.Fatalf("read modified GameUserSettings.ini: %v", err)
	}
	modifiedStr := string(modifiedData)

	if !strings.Contains(modifiedStr, "FrameRateLimit=1.000000") {
		t.Errorf("expected FrameRateLimit=1.000000, content:\n%s", modifiedStr)
	}
	if !strings.Contains(modifiedStr, "bUseVSync=False") {
		t.Errorf("expected bUseVSync=False, content:\n%s", modifiedStr)
	}
	if strings.Contains(modifiedStr, "[/Script/Engine.GameUserSettings]") {
		t.Errorf("absent Engine user settings section was invented, content:\n%s", modifiedStr)
	}
	if !strings.Contains(modifiedStr, "ResolutionSizeX=640") || !strings.Contains(modifiedStr, "ResolutionSizeY=480") {
		t.Errorf("expected 640x480 resolution, content:\n%s", modifiedStr)
	}
	if !strings.Contains(modifiedStr, "sg.ResolutionQuality=25.000000") {
		t.Errorf("expected sg.ResolutionQuality=25.000000, content:\n%s", modifiedStr)
	}
	if !strings.Contains(modifiedStr, "CustomUserOption=PreserveMePlease") {
		t.Errorf("expected custom user option to be preserved")
	}

	// Verify file is writable so Unreal Engine does not fail with ERROR_ACCESS_DENIED and revert to defaults
	fi, statErr := os.Stat(gameUserSettingsPath)
	if statErr != nil {
		t.Fatalf("stat gameUserSettingsPath: %v", statErr)
	}
	if fi.Mode().Perm()&0o200 == 0 {
		t.Errorf("expected file to remain writable for Unreal Engine, perm=%o", fi.Mode().Perm())
	}

	// CRITICAL INVARIANT: Check that Input.ini was NOT touched at all
	currentInputData, err := os.ReadFile(inputIniPath)
	if err != nil {
		t.Fatalf("read Input.ini: %v", err)
	}
	currentInputHash := sha256.Sum256(currentInputData)
	if !bytes.Equal(originalInputHash[:], currentInputHash[:]) {
		t.Fatalf("SECURITY VIOLATION: Input.ini was modified! Controls must never be touched.")
	}

	// Reapplying during an active transaction must leave the original backup intact.
	if err := mgr.ApplyPotatoConfig(5, true); err == nil {
		t.Fatal("second ApplyPotatoConfig should refuse an active transaction")
	}
	backupData, err := os.ReadFile(mgr.BackupPath())
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if !strings.Contains(string(backupData), "FrameRateLimit=144.000000") {
		t.Fatalf("backup was corrupted/overwritten with potato settings!")
	}

	// 5. Restore original configuration
	if err := mgr.RestoreOriginalConfig(); err != nil {
		t.Fatalf("RestoreOriginalConfig failed: %v", err)
	}

	restoredData, err := os.ReadFile(gameUserSettingsPath)
	if err != nil {
		t.Fatalf("read restored GameUserSettings.ini: %v", err)
	}

	if !bytes.Equal(originalUserSettings, restoredData) {
		t.Fatalf("restored data does not match original byte-for-byte!\nOriginal:\n%s\nRestored:\n%s", string(originalUserSettings), string(restoredData))
	}

	restoredFi, statErr := os.Stat(gameUserSettingsPath)
	if statErr != nil {
		t.Fatalf("stat restored GameUserSettings.ini: %v", statErr)
	}
	if restoredFi.Mode().Perm()&0o200 == 0 {
		t.Errorf("expected restored file to be writable, got perm=%o", restoredFi.Mode().Perm())
	}

	if mgr.HasActiveBackup() {
		t.Fatal("restored backup must no longer be marked active")
	}
	if _, err := os.Stat(mgr.BackupPath()); err != nil {
		t.Fatalf("expected archived backup to remain available: %v", err)
	}

	status = mgr.GetStatus()
	if status.IsPotatoMode {
		t.Fatalf("expected IsPotatoMode=false after restore")
	}
	if status.CurrentFpsCap != 144 {
		t.Fatalf("expected FPS cap 144 after restore, got %g", status.CurrentFpsCap)
	}

	// 6. Test SaveCurrentAsBackup:
	// A) When in normal mode, user changes graphics and saves as new baseline
	newUserSetting := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=165.000000\nResolutionSizeX=3840\nResolutionSizeY=2160\nFullscreenMode=0\n")
	if err := os.WriteFile(gameUserSettingsPath, newUserSetting, 0o644); err != nil {
		t.Fatalf("write new user setting: %v", err)
	}
	if err := mgr.SaveCurrentAsBackup(false); err != nil {
		t.Fatalf("SaveCurrentAsBackup failed: %v", err)
	}
	updatedBackup, _ := os.ReadFile(mgr.BackupPath())
	if !strings.Contains(string(updatedBackup), "FrameRateLimit=165.000000") {
		t.Fatalf("backup was not updated with new 165 FPS settings")
	}

	// B) When in potato mode, SaveCurrentAsBackup without force MUST FAIL to protect user
	if err := mgr.ApplyPotatoConfig(1, true); err != nil {
		t.Fatalf("apply Potato Mode to updated settings: %v", err)
	}
	if err := mgr.SaveCurrentAsBackup(false); err == nil {
		t.Fatalf("SaveCurrentAsBackup should have rejected overwriting backup while in potato mode!")
	}

	// Restore back to updated baseline (165 FPS)
	_ = mgr.RestoreOriginalConfig()
	finalRestored, _ := os.ReadFile(gameUserSettingsPath)
	if !strings.Contains(string(finalRestored), "FrameRateLimit=165.000000") {
		t.Fatalf("restored config does not match updated baseline: %s", string(finalRestored))
	}

	// Check Input.ini one more time
	finalInputData, _ := os.ReadFile(inputIniPath)
	if !bytes.Equal(originalInputHash[:], sha256Sum(finalInputData)) {
		t.Fatalf("SECURITY VIOLATION: Input.ini was touched during restore!")
	}
}

func TestSquadConfigManager_PreservesUnverifiedLegacyBackup(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "Config", "WindowsClient")
	legacyConfigDir := filepath.Join(tempDir, "Config", "WindowsNoEditor")
	_ = os.MkdirAll(configDir, 0o755)
	_ = os.MkdirAll(legacyConfigDir, 0o755)

	primaryConfig := filepath.Join(configDir, "GameUserSettings.ini")
	legacyConfig := filepath.Join(legacyConfigDir, "GameUserSettings.ini")
	backupDir := filepath.Join(tempDir, "backup")
	_ = os.MkdirAll(backupDir, 0o755)

	// User has a real 3KB config in WindowsClient
	realUserConfig := []byte("[/Script/Engine.GameUserSettings]\nFrameRateLimit=144.000000\nResolutionSizeX=1920\nResolutionSizeY=1080\n")
	_ = os.WriteFile(primaryConfig, realUserConfig, 0o644)

	// An older Sentry version created a dummy 287-byte file in WindowsNoEditor and backed it up
	dummyPotato := []byte("[/Script/Engine.GameUserSettings]\nFrameRateLimit=1.000000\nbUseVSync=False\n")
	_ = os.WriteFile(legacyConfig, dummyPotato, 0o644)

	staleBackupFile := filepath.Join(backupDir, "GameUserSettings.ini.original")
	staleManifestFile := filepath.Join(backupDir, "backup_manifest.json")
	_ = os.WriteFile(staleBackupFile, dummyPotato, 0o600)
	_ = os.WriteFile(staleManifestFile, []byte(`{"originalPath":"`+strings.ReplaceAll(legacyConfig, `\`, `\\`)+`"}`), 0o600)

	// An old backup must never be deleted solely because its path or size looks stale.
	mgr := NewConfigManagerWithPaths(primaryConfig, backupDir)

	if mgr.HasActiveBackup() {
		t.Fatal("legacy backup without an active transaction must not be marked active")
	}

	if _, err := os.Stat(staleBackupFile); err != nil {
		t.Fatalf("legacy backup was removed: %v", err)
	}

	// Now apply potato mode: it should safely backup the REAL config (primaryConfig)
	if err := mgr.ApplyPotatoConfig(1, false); err != nil {
		t.Fatalf("ApplyPotatoConfig failed: %v", err)
	}
	archives, err := filepath.Glob(filepath.Join(backupDir, "legacy-*", "GameUserSettings.ini.original"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("legacy backup was not archived before replacement: %v, %v", archives, err)
	}

	if !mgr.HasActiveBackup() {
		t.Fatalf("expected active backup after applying potato mode to real config")
	}

	// Restore: primaryConfig must have original 144 FPS
	if err := mgr.RestoreOriginalConfig(); err != nil {
		t.Fatalf("RestoreOriginalConfig failed: %v", err)
	}

	restoredData, _ := os.ReadFile(primaryConfig)
	if !strings.Contains(string(restoredData), "FrameRateLimit=144.000000") {
		t.Fatalf("restored data did not preserve real user config: %s", string(restoredData))
	}
}

func TestSquadConfigManager_MigratesVerifiedLegacyPotatoSession(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	backupDir := filepath.Join(root, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	patched := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=1.000000\nbUseVSync=False\n[SystemSettings]\nt.MaxFPS=1.000000\n")
	if err := os.WriteFile(primary, patched, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "GameUserSettings.ini.original"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := json.Marshal(BackupManifest{OriginalPath: primary, Sha256: checksum(original), SizeBytes: int64(len(original))})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "backup_manifest.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(primary, backupDir)
	if !manager.HasActiveBackup() {
		t.Fatal("verified legacy potato session was not adopted for recovery")
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatalf("restore migrated legacy backup: %v", err)
	}
	current, err := os.ReadFile(primary)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("legacy restore did not recover original bytes: %v", err)
	}
}

func TestPatchPotatoIni_EngineCVarUsesEngineIni(t *testing.T) {
	original := []byte("[/Script/Engine.GameUserSettings]\nFrameRateLimit=120.000000\nbUseVSync=True\n")
	patched := patchPotatoIni(original, 1, false)
	patchedStr := string(patched)

	if !strings.Contains(patchedStr, "FrameRateLimit=1.000000") {
		t.Fatalf("missing FrameRateLimit in patched: %s", patchedStr)
	}
	if strings.Contains(patchedStr, "t.MaxFPS=") {
		t.Fatalf("t.MaxFPS must not be written to GameUserSettings.ini: %s", patchedStr)
	}
	engineOriginal := []byte("[SystemSettings]\r\nr.Streaming.PoolSize=1000\r\n")
	if !strings.Contains(string(patchEngineIni(engineOriginal, 1)), "r.Streaming.PoolSize=150") {
		t.Fatal("missing r.Streaming.PoolSize in Engine.ini patch")
	}
	if !bytes.Contains(patchEngineIni(engineOriginal, 1), []byte("\r\n")) {
		t.Fatal("existing Engine.ini CRLF line endings were lost")
	}
	if len(patchEngineIni(nil, 1)) != 0 {
		t.Fatal("missing Engine.ini settings were invented")
	}
	if !strings.Contains(patchedStr, "bUseVSync=False") {
		t.Fatalf("missing bUseVSync in patched: %s", patchedStr)
	}
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func TestCheckMatchesPotatoSettings(t *testing.T) {
	normalConfig := []byte("[/Script/Engine.GameUserSettings]\nFrameRateLimit=144.000000\nResolutionSizeX=1920\nResolutionSizeY=1080\n")
	potatoConfig := []byte("[/Script/Engine.GameUserSettings]\nFrameRateLimit=1.000000\nResolutionSizeX=640\nResolutionSizeY=480\n")

	// 1. Normal config should NOT match 1 FPS + low resolution
	matches, err := CheckMatchesPotatoSettings(normalConfig, 1, true)
	if err != nil {
		t.Fatalf("unexpected error on normal config: %v", err)
	}
	if matches {
		t.Fatalf("normal config should not match potato settings")
	}

	// 2. Potato config should match 1 FPS + low resolution
	matches, err = CheckMatchesPotatoSettings(potatoConfig, 1, true)
	if err != nil {
		t.Fatalf("unexpected error on potato config: %v", err)
	}
	if !matches {
		t.Fatalf("potato config should match 1 FPS + low resolution")
	}

	// 3. Potato config with 30 FPS requested should NOT match 1 FPS
	matches, err = CheckMatchesPotatoSettings(potatoConfig, 30, true)
	if err != nil {
		t.Fatalf("unexpected error on potato config: %v", err)
	}
	if matches {
		t.Fatalf("potato config with 1 FPS should not match requested 30 FPS")
	}

	// 4. Test SetShowEmptyServers without existing file
	tempDir2 := t.TempDir()
	filePath2 := filepath.Join(tempDir2, "DoesNotExist.ini")
	mgr2 := NewConfigManagerWithPaths(filePath2, filepath.Join(tempDir2, "backup"))
	if err := mgr2.SetShowEmptyServers(true); err != nil {
		t.Fatalf("SetShowEmptyServers failed for non-existent file: %v", err)
	}
	content, err := os.ReadFile(filePath2)
	if err != nil {
		t.Fatalf("file should have been created: %v", err)
	}
	if !strings.Contains(string(content), "bShowEmptyServers=True") {
		t.Fatalf("file does not contain bShowEmptyServers=True: %s", string(content))
	}

	// 5. Test on ConfigManager with file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "GameUserSettings.ini")
	if err := os.WriteFile(filePath, potatoConfig, 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := NewConfigManagerWithPaths(filePath, filepath.Join(tempDir, "backup"))
	matches, err = mgr.CheckMatchesPotatoSettings(1, true)
	if err != nil || !matches {
		t.Fatalf("ConfigManager.CheckMatchesPotatoSettings failed: matches=%v, err=%v", matches, err)
	}
}

