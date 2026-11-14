package game

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPotatoTransactionRejectsUnknownOrIncompleteSettings(t *testing.T) {
	for _, test := range []struct {
		name     string
		original string
		isLowRes bool
	}{
		{name: "unknown-section", original: "[NewSquadSettings]\nFrameRateLimit=120.000000\n"},
		{name: "missing-fps", original: "[/Script/Squad.SquadGameUserSettings]\nCustomSetting=Keep\n"},
		{name: "duplicate-fps", original: "[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=120.000000\nFrameRateLimit=90.000000\n"},
		{name: "invalid-fps", original: "[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=NaN\n"},
		{name: "invalid-vsync", original: "[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=120.000000\nbUseVSync=Unknown\n"},
		{name: "invalid-resolution", original: "[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=120.000000\nResolutionSizeX=Unknown\nResolutionSizeY=1080\nFullscreenMode=0\n", isLowRes: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			primary := filepath.Join(root, "GameUserSettings.ini")
			original := []byte(test.original)
			if err := os.WriteFile(primary, original, 0o644); err != nil {
				t.Fatal(err)
			}
			manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
			if err := manager.BackupConfig(); err != nil {
				t.Fatal(err)
			}
			if err := manager.ApplyPotatoConfig(1, test.isLowRes); err == nil {
				t.Fatal("missing or ambiguous game settings were replaced with guessed values")
			}
			current, err := os.ReadFile(primary)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("rejected settings were changed: %v", err)
			}
			backup, err := os.ReadFile(manager.BackupPath())
			if err != nil || !bytes.Equal(backup, original) {
				t.Fatalf("original backup was changed: %v", err)
			}
			if manager.HasActiveBackup() {
				t.Fatal("rejected attempt left a pending transaction")
			}
			if _, err := os.Stat(filepath.Join(root, "Engine.ini")); !os.IsNotExist(err) {
				t.Fatalf("rejected attempt invented Engine.ini: %v", err)
			}
		})
	}
}

func TestPotatoTransactionInjectsMissingResolution(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=120.000000\r\n")
	if err := os.WriteFile(primary, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(1, true); err != nil {
		t.Fatalf("ApplyPotatoConfig failed when resolution was missing from original: %v", err)
	}
	patched, err := os.ReadFile(primary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), "ResolutionSizeX=640") || !strings.Contains(string(patched), "ResolutionSizeY=480") {
		t.Fatalf("low resolution was not injected: %s", patched)
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatalf("RestoreOriginalConfig failed: %v", err)
	}
	restored, err := os.ReadFile(primary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("restoration did not remove injected resolution: %s", restored)
	}
}

func TestPotatoTransactionPreservesRecordedOriginalPermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0o600, 0o400} {
		for _, isMissing := range []bool{false, true} {
			name := mode.String()
			if isMissing {
				name += "-missing"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "GameUserSettings.ini")
				original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
				if err := os.WriteFile(path, original, mode); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
				if err := manager.ApplyPotatoConfig(30, false); err != nil {
					t.Fatal(err)
				}
				if isMissing {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				// Reopen from the persisted transaction, as recovery after a restart does.
				manager = NewConfigManagerWithPaths(path, manager.backupDir)
				if err := manager.RestoreOriginalConfig(); err != nil {
					t.Fatal(err)
				}
				after, err := os.Stat(path)
				if err != nil || after.Mode().Perm() != before.Mode().Perm() {
					t.Fatalf("original permissions were replaced: before=%o after=%v error=%v", before.Mode().Perm(), after, err)
				}
				data, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, original) {
					t.Fatalf("original bytes were lost: %v", err)
				}
			})
		}
	}
}

func TestPotatoRecoveryPreservesExternalPermissionChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission classes are not exposed on Windows")
	}
	root := t.TempDir()
	path := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	if err := os.WriteFile(path, original, 0o400); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(30, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatal("recovery replaced permissions changed outside Sentry")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("recovery lost original settings: %v", err)
	}
}

func TestPotatoTransactionDoesNotInventMissingGameSettings(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=144.000000\r\nCustomOption=Keep\r\n")
	if err := os.WriteFile(primary, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(primary)
	want := bytes.Replace(original, []byte("FrameRateLimit=144.000000"), []byte("FrameRateLimit=1.000000"), 1)
	if err != nil || !bytes.Equal(current, want) {
		t.Fatalf("unavailable game settings were invented: %q, error %v", current, err)
	}
	if _, err := os.Stat(filepath.Join(root, "Engine.ini")); !os.IsNotExist(err) {
		t.Fatalf("missing Engine.ini was invented: %v", err)
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	current, err = os.ReadFile(primary)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("verified original was not restored: %v", err)
	}
}

func TestPotatoTransactionRollsBackWhenAConfigCannotBeWritten(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions enforced for a non-root user")
	}
	root := t.TempDir()
	primaryDir := filepath.Join(root, "primary")
	secondaryDir := filepath.Join(root, "secondary")
	for _, dir := range []string{primaryDir, secondaryDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(primaryDir, "GameUserSettings.ini")
	secondary := filepath.Join(secondaryDir, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	for _, path := range []string{primary, secondary} {
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(secondaryDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(secondaryDir, 0o700); err != nil {
			t.Errorf("restore test directory permissions: %v", err)
		}
	})
	manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
	manager.secondaryPaths = []string{secondary}
	if err := manager.ApplyPotatoConfig(1, false); err == nil {
		t.Fatal("unwritable config did not stop Potato Mode")
	}
	for _, path := range []string{primary, secondary} {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatalf("partial patch was not rolled back for %s: %v", path, err)
		}
	}
	if manager.HasActiveBackup() {
		t.Fatal("successful rollback left a pending transaction")
	}
}

func TestPotatoTransactionRestoresEveryModifiedConfigAfterRestart(t *testing.T) {
	root := t.TempDir()
	primaryDir := filepath.Join(root, "Config", "WindowsClient")
	secondaryDir := filepath.Join(root, "Config", "WindowsNoEditor")
	for _, dir := range []string{primaryDir, secondaryDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(primaryDir, "GameUserSettings.ini")
	secondary := filepath.Join(secondaryDir, "GameUserSettings.ini")
	primaryEngine := filepath.Join(primaryDir, "Engine.ini")
	secondaryEngine := filepath.Join(secondaryDir, "Engine.ini")
	primaryOriginal := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=144.000000\r\nCustomOption=KeepMe\r\n")
	secondaryOriginal := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=60.000000\n")
	engineOriginal := []byte("[SystemSettings]\r\nt.MaxFPS=144.000000\r\nr.CustomSetting=7\r\n")
	for path, data := range map[string][]byte{
		primary: primaryOriginal, secondary: secondaryOriginal, primaryEngine: engineOriginal,
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	backupDir := filepath.Join(root, "backup")
	manager := NewConfigManagerWithPaths(primary, backupDir)
	manager.secondaryPaths = []string{secondary}
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatalf("apply Potato Mode: %v", err)
	}
	// Existing 1.2.2 transactions may own files created by the old implementation.
	addLegacyCreatedEngine(t, manager, secondaryEngine)
	for _, path := range []string{primary, secondary} {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "FrameRateLimit=1.000000") {
			t.Fatalf("FPS patch missing in %s: %v", path, err)
		}
	}
	for _, path := range []string{primaryEngine, secondaryEngine} {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "r.Streaming.PoolSize=150") {
			t.Fatalf("engine patch missing in %s: %v", path, err)
		}
	}
	if bytes.Contains(primaryOriginal, []byte("\n")) {
		patched, err := os.ReadFile(primary)
		if err != nil || !bytes.Contains(patched, []byte("\r\n")) {
			t.Fatalf("CRLF was not retained in primary settings: %v", err)
		}
	}

	// A new manager must recover the same transaction after a Sentry restart.
	restarted := NewConfigManagerWithPaths(primary, backupDir)
	restarted.secondaryPaths = []string{secondary}
	if !restarted.HasActiveBackup() {
		t.Fatal("active transaction was lost after restart")
	}
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatalf("restore after restart: %v", err)
	}
	for path, original := range map[string][]byte{
		primary: primaryOriginal, secondary: secondaryOriginal, primaryEngine: engineOriginal,
	} {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatalf("%s was not restored exactly: %v", path, err)
		}
	}
	if _, err := os.Stat(secondaryEngine); !os.IsNotExist(err) {
		t.Fatalf("Engine.ini created for Potato Mode was not removed: %v", err)
	}
	if restarted.HasActiveBackup() {
		t.Fatal("transaction remained active after restore")
	}
	userUpdated := []byte("[UserSettings]\nNewGraphicsChoice=1\n")
	if err := os.WriteFile(primary, userUpdated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(primary)
	if err != nil || !bytes.Equal(current, userUpdated) {
		t.Fatal("inactive archived backup overwrote newer user settings")
	}
}

func TestPotatoTransactionRecoversMissingConfigsAfterRestart(t *testing.T) {
	for _, test := range []struct {
		name          string
		missingName   string
		hasEngineFile bool
	}{
		{name: "existing-engine", missingName: "Engine.ini", hasEngineFile: true},
		{name: "settings", missingName: "GameUserSettings.ini", hasEngineFile: true},
		{name: "created-engine", missingName: "Engine.ini"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			primary := filepath.Join(root, "GameUserSettings.ini")
			engine := filepath.Join(root, "Engine.ini")
			originals := map[string][]byte{
				primary: []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=144.000000\r\nResolutionSizeX=1920\r\nResolutionSizeY=1080\r\nFullscreenMode=0\r\n"),
			}
			if test.hasEngineFile {
				originals[engine] = []byte("[SystemSettings]\r\nt.MaxFPS=144.000000\r\nr.CustomSetting=7\r\n")
			}
			for path, data := range originals {
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			backupDir := filepath.Join(root, "backup")
			manager := NewConfigManagerWithPaths(primary, backupDir)
			if err := manager.ApplyPotatoConfig(1, true); err != nil {
				t.Fatal(err)
			}
			if !test.hasEngineFile {
				addLegacyCreatedEngine(t, manager, engine)
			}
			if err := os.Remove(filepath.Join(root, test.missingName)); err != nil {
				t.Fatal(err)
			}

			restarted := NewConfigManagerWithPaths(primary, backupDir)
			if err := restarted.RestoreOriginalConfig(); err != nil {
				t.Fatalf("restore after config disappeared: %v", err)
			}
			for path, original := range originals {
				current, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(current, original) {
					t.Fatalf("%s was not restored exactly: %v", path, err)
				}
			}
			if !test.hasEngineFile {
				if _, err := os.Stat(engine); !os.IsNotExist(err) {
					t.Fatalf("absent original Engine.ini was recreated: %v", err)
				}
			}
			if restarted.HasActiveBackup() {
				t.Fatal("restoration still blocks a new session")
			}
			if err := restarted.RestoreOriginalConfig(); err != nil {
				t.Fatalf("repeat restoration: %v", err)
			}
			if err := restarted.ApplyPotatoConfig(1, false); err != nil {
				t.Fatalf("start a new Potato Mode session: %v", err)
			}
			if err := restarted.RestoreOriginalConfig(); err != nil {
				t.Fatalf("restore the new session: %v", err)
			}
		})
	}
}

func TestPotatoTransactionRejectsCorruptBackupForMissingConfig(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	engine := filepath.Join(root, "Engine.ini")
	for path, data := range map[string]string{
		primary: "[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n",
		engine:  "[SystemSettings]\nt.MaxFPS=144.000000\n",
	} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	backupDir := filepath.Join(root, "backup")
	manager := NewConfigManagerWithPaths(primary, backupDir)
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(primary)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := manager.readManifestLocked()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Files {
		if entry.Path == engine {
			if err := os.WriteFile(filepath.Join(backupDir, entry.BackupName), []byte("corrupt"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Remove(engine); err != nil {
		t.Fatal(err)
	}
	restarted := NewConfigManagerWithPaths(primary, backupDir)
	if err := restarted.RestoreOriginalConfig(); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("corrupt backup for missing config was accepted: %v", err)
	}
	current, err := os.ReadFile(primary)
	if err != nil || !bytes.Equal(current, patched) {
		t.Fatalf("failed validation changed other settings: %v", err)
	}
	if _, err := os.Stat(engine); !os.IsNotExist(err) {
		t.Fatalf("corrupt Engine.ini was recreated: %v", err)
	}
	if !restarted.HasActiveBackup() {
		t.Fatal("failed recovery discarded the pending transaction")
	}
}

func TestPotatoTransactionRejectsCorruptBackupAndPreservesPlayerChanges(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	if err := os.WriteFile(primary, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.BackupPath(), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreOriginalConfig(); err == nil {
		t.Fatal("corrupt backup was accepted")
	}
	if err := os.WriteFile(manager.BackupPath(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(primary)
	if err != nil {
		t.Fatal(err)
	}
	changed := append(patched, []byte("\nPlayerChanged=1\n")...)
	if err := os.WriteFile(primary, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatalf("player changes could not be merged: %v", err)
	}
	current, err := os.ReadFile(primary)
	if err != nil || !strings.Contains(string(current), "PlayerChanged=1") || !strings.Contains(string(current), "FrameRateLimit=144.000000") {
		t.Fatal("restore did not retain the player's new key and original FPS limit")
	}
}

func TestPotatoTransactionRequiresExistingSquadSettings(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(1, false); err == nil {
		t.Fatal("Potato Mode created a dummy GameUserSettings.ini")
	}
	if _, err := os.Stat(primary); !os.IsNotExist(err) {
		t.Fatalf("unexpected dummy settings file: %v", err)
	}
}

func TestPotatoTransactionPreservesGameChangesToCreatedEngineIni(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "GameUserSettings.ini")
	engine := filepath.Join(root, "Engine.ini")
	if err := os.WriteFile(primary, []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=120.000000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewConfigManagerWithPaths(primary, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	addLegacyCreatedEngine(t, manager, engine)
	if err := os.WriteFile(engine, []byte("[SystemSettings]\nr.Streaming.PoolSize=150\n+CustomArray=A\n+CustomArray=B\nGameSetting=Keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreOriginalConfig(); err != nil {
		t.Fatalf("restore game-edited Engine.ini: %v", err)
	}
	data, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "r.Streaming.PoolSize=") || !strings.Contains(string(data), "GameSetting=Keep") || !strings.Contains(string(data), "+CustomArray=B") {
		t.Fatalf("restoration lost game changes or left streaming pool patch: %s", data)
	}
}

// addLegacyCreatedEngine reproduces an active 1.2.2 transaction without using new patch logic.
func addLegacyCreatedEngine(t *testing.T, manager *ConfigManager, path string) {
	t.Helper()
	patched := []byte("[SystemSettings]\r\nr.Streaming.PoolSize=150\r\n")
	name := "legacy-created-Engine.ini.patched"
	if err := os.WriteFile(path, patched, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manager.backupDir, name), patched, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := manager.readManifestLocked()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Files = append(manifest.Files, BackupEntry{
		Path: path, PatchedName: name, PatchedSha256: checksum(patched), Existed: false,
	})
	if err := manager.writeManifestLocked(manifest); err != nil {
		t.Fatal(err)
	}
}

func TestConfigManagerRecoversActiveSessionWhenSquadChangesConfigDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "app-config"))
	oldDir := filepath.Join(root, "SquadGame", "Saved", "Config", "WindowsNoEditor")
	newDir := filepath.Join(root, "SquadGame", "Saved", "Config", "WindowsClient")
	for _, dir := range []string{oldDir, newDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldPath := filepath.Join(oldDir, "GameUserSettings.ini")
	newPath := filepath.Join(newDir, "GameUserSettings.ini")
	// Discovery must remain inside this fixture even when Squad is installed.
	discover := func() (string, []string) {
		return selectSquadConfigPaths([]string{newPath, oldPath})
	}
	backupDir := filepath.Join(root, "backup")
	oldOriginal := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=144.000000\r\n")
	if err := os.WriteFile(oldPath, oldOriginal, 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := newConfigManager(backupDir, discover)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, bytes.Repeat([]byte("PlayerSetting=1\r\n"), 100), 0o644); err != nil {
		t.Fatal(err)
	}
	restarted, err := newConfigManager(backupDir, discover)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.ConfigPath() != oldPath || !restarted.HasActiveBackup() {
		t.Fatalf("active transaction path was lost: %s", restarted.ConfigPath())
	}
	if err := restarted.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(oldPath)
	if err != nil || !bytes.Equal(restored, oldOriginal) {
		t.Fatalf("old active config was not restored: %v", err)
	}
	if restarted.ConfigPath() != newPath {
		t.Fatalf("new config path was not detected after restore: %s", restarted.ConfigPath())
	}
}
