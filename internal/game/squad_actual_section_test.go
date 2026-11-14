package game

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// This section and key spelling were observed in a real Squad-generated config.
func TestActualSquadSectionStatusPatchAndRecovery(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SQGameUserSettings]\r\nFrameRateLimit=90.000000\r\nMenuFrameRateLimit=60.000000\r\nbUseVSync=False\r\nResolutionSizeX=1280\r\nResolutionSizeY=720\r\nDesiredScreenWidth=1280\r\nDesiredScreenHeight=720\r\nFullscreenMode=0\r\nPlayerOption=Keep\r\n[ScalabilityGroups]\r\nsg.ResolutionQuality=50\r\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "Input.ini")
	if err := os.WriteFile(input, []byte("Bindings=Keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	status := manager.GetStatus()
	if status.ErrorMessage != "" || !status.ConfigFound || status.CurrentFpsCap != 90 {
		t.Fatalf("real Squad section was not recognized: %+v", status)
	}
	if err := manager.ApplyPotatoConfig(15, true); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := iniSettings(patched)
	for key, expected := range map[string]string{
		"frameratelimit":      "15.000000",
		"menuframeratelimit":  "60.000000",
		"resolutionsizex":     "640",
		"resolutionsizey":     "480",
		"desiredscreenwidth":  "640",
		"desiredscreenheight": "480",
		"playeroption":        "Keep",
	} {
		if settings["/script/squad.sqgameusersettings\x00"+key].value != expected {
			t.Fatalf("incorrect patch for %s", key)
		}
	}
	restarted := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("original data did not return: %v", err)
	}
	bindings, err := os.ReadFile(input)
	if err != nil || string(bindings) != "Bindings=Keep\n" {
		t.Fatalf("Input.ini changed: %v", err)
	}
}


func TestActualSquadSectionConflictingFpsRemainsAnError(t *testing.T) {
	data := []byte("[/Script/Engine.GameUserSettings]\nFrameRateLimit=30\n[/Script/Squad.SQGameUserSettings]\nFrameRateLimit=90\n")
	if _, err := observedFpsCap(data); err == nil {
		t.Fatal("conflicting supported sections were accepted")
	}
}

func TestActualSquadSectionLegacyPotatoDetection(t *testing.T) {
	data := []byte("[/Script/Squad.SQGameUserSettings]\nFrameRateLimit=1\nbUseVSync=False\n[ScalabilityGroups]\nsg.ResolutionQuality=25\n")
	if !looksLikePotatoConfig(data) {
		t.Fatal("legacy Potato settings could be mistaken for original settings")
	}
}

// Windows Squad often splits resolution into Engine.GameUserSettings and FPS into Squad.SquadGameUserSettings
func TestWindowsSquadSplitSectionResolutionAndFps(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Engine.GameUserSettings]\r\nResolutionSizeX=1920\r\nResolutionSizeY=1080\r\nFullscreenMode=1\r\n[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=60.000000\r\nMenuFrameRateLimit=60.000000\r\nbUseVSync=False\r\n[ScalabilityGroups]\r\nsg.ResolutionQuality=100.000000\r\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	status := manager.GetStatus()
	if status.ErrorMessage != "" || !status.ConfigFound || status.CurrentFpsCap != 60 {
		t.Fatalf("Windows split-section settings not recognized: %+v", status)
	}

	if err := manager.ApplyPotatoConfig(1, true); err != nil {
		t.Fatalf("ApplyPotatoConfig failed on Windows split-section: %v", err)
	}

	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := iniSettings(patched)

	if settings["/script/engine.gameusersettings\x00resolutionsizex"].value != "640" {
		t.Errorf("ResolutionSizeX not patched in Engine.GameUserSettings: got %s", settings["/script/engine.gameusersettings\x00resolutionsizex"].value)
	}
	if settings["/script/engine.gameusersettings\x00resolutionsizey"].value != "480" {
		t.Errorf("ResolutionSizeY not patched in Engine.GameUserSettings: got %s", settings["/script/engine.gameusersettings\x00resolutionsizey"].value)
	}
	if settings["/script/squad.squadgameusersettings\x00frameratelimit"].value != "1.000000" {
		t.Errorf("FrameRateLimit not patched in Squad.SquadGameUserSettings: got %s", settings["/script/squad.squadgameusersettings\x00frameratelimit"].value)
	}
	if settings["/script/squad.squadgameusersettings\x00menuframeratelimit"].value != "60.000000" {
		t.Errorf("MenuFrameRateLimit not patched in Squad.SquadGameUserSettings: got %s", settings["/script/squad.squadgameusersettings\x00menuframeratelimit"].value)
	}

	matches, err := CheckMatchesPotatoSettings(patched, 1, true)
	if err != nil || !matches {
		t.Fatalf("CheckMatchesPotatoSettings failed: matches=%v, err=%v", matches, err)
	}

	restarted := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatalf("RestoreOriginalConfig failed: %v", err)
	}
	restored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("restored data did not match original: %v", err)
	}
}

// TestWindowsSquadStaleSecondaryConfigIgnored reproduces Ssan4oyss's exact setup:
// Primary config in Windows has FPS only, stale config in WindowsNoEditor has no resolution settings.
func TestWindowsSquadStaleSecondaryConfigIgnored(t *testing.T) {
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Windows", "GameUserSettings.ini")
	secondaryPath := filepath.Join(root, "WindowsNoEditor", "GameUserSettings.ini")
	_ = os.MkdirAll(filepath.Dir(primaryPath), 0o755)
	_ = os.MkdirAll(filepath.Dir(secondaryPath), 0o755)

	primaryOriginal := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=60.000000\r\nMenuFrameRateLimit=60.000000\r\n")
	secondaryOriginal := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=60.000000\r\n")
	if err := os.WriteFile(primaryPath, primaryOriginal, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondaryPath, secondaryOriginal, 0o600); err != nil {
		t.Fatal(err)
	}

	manager := NewConfigManagerWithPaths(primaryPath, filepath.Join(root, "backup"))
	manager.secondaryPaths = []string{secondaryPath}

	if err := manager.ApplyPotatoConfig(1, true); err != nil {
		t.Fatalf("ApplyPotatoConfig failed when secondary had no resolution: %v", err)
	}

	patchedPrimary, err := os.ReadFile(primaryPath)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := iniSettings(patchedPrimary)
	if settings["/script/engine.gameusersettings\x00resolutionsizex"].value != "640" &&
		settings["/script/squad.squadgameusersettings\x00resolutionsizex"].value != "640" {
		t.Fatalf("low resolution was not injected into primary config: %s", patchedPrimary)
	}

	matches, err := CheckMatchesPotatoSettings(patchedPrimary, 1, true)
	if err != nil || !matches {
		t.Fatalf("CheckMatchesPotatoSettings failed: %v", err)
	}

	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatalf("RestoreOriginalConfig failed: %v", err)
	}
	restored, err := os.ReadFile(primaryPath)
	if err != nil || !bytes.Equal(restored, primaryOriginal) {
		t.Fatalf("restored primary did not match original: %s", restored)
	}
}
