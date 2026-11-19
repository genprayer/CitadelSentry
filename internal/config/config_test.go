package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestSettings(t *testing.T, settings AppSettings) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("APPDATA", root)
	directory := filepath.Join(root, "citadel-sentry")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.json")
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, data
}

func TestConfigLoadPreservesExplicitAPIEndpoints(t *testing.T) {
	for _, endpoint := range []string{"https://citadel.example.com", "https://join-the-citadel.com"} {
		t.Run(endpoint, func(t *testing.T) {
			settings := DefaultSettings()
			settings.WebApiBaseUrl = endpoint
			path, original := writeTestSettings(t, settings)
			manager, err := NewManager()
			if err != nil {
				t.Fatal(err)
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, original) || manager.Get().WebApiBaseUrl != endpoint {
				t.Fatalf("user endpoint was replaced: saved=%q error=%v", manager.Get().WebApiBaseUrl, err)
			}
		})
	}
}

func TestInitialPlatformChoicesApplyOnlyToMissingSettingsFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("APPDATA", root)
	initial := DefaultSettings()
	initial.Potato.LowCpuPriority = false
	manager, err := NewManagerWithDefaults(initial)
	if err != nil {
		t.Fatal(err)
	}
	if manager.Get().Potato.LowCpuPriority {
		t.Fatal("fresh settings requested unsupported priority tuning")
	}
	explicit := DefaultSettings()
	data, err := json.Marshal(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.filePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err = NewManagerWithDefaults(initial)
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(manager.filePath)
	if err != nil || !bytes.Equal(current, data) || !manager.Get().Potato.LowCpuPriority {
		t.Fatalf("platform defaults overwrote an explicit stored choice: %v", err)
	}
}

func TestConfigInvalidValuesDoNotTurnIntoSavedDefaults(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*AppSettings)
	}{
		{name: "missing endpoint", change: func(s *AppSettings) { s.WebApiBaseUrl = "" }},
		{name: "unsupported protocol", change: func(s *AppSettings) { s.WebApiBaseUrl = "file:///settings" }},
		{name: "empty query", change: func(s *AppSettings) { s.WebApiBaseUrl = "https://example.invalid/?" }},
		{name: "empty fragment", change: func(s *AppSettings) { s.WebApiBaseUrl = "https://example.invalid/#" }},
		{name: "missing target", change: func(s *AppSettings) { s.Seed.DefaultTargetPlayers = 0 }},
		{name: "invalid duration", change: func(s *AppSettings) { s.Seed.DefaultMaxDurationMinutes = -1 }},
		{name: "invalid FPS", change: func(s *AppSettings) { s.Potato.FpsLimit = 0 }},
		{name: "excess FPS", change: func(s *AppSettings) { s.Potato.FpsLimit = 100 }},
		{name: "unknown action", change: func(s *AppSettings) { s.Seed.DefaultPostSeedAction = "unknown" }},
		{name: "unknown language", change: func(s *AppSettings) { s.General.Language = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultSettings()
			test.change(&settings)
			path, original := writeTestSettings(t, settings)
			if _, err := NewManager(); err == nil {
				t.Fatal("invalid value was loaded using guessed defaults")
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("rejected settings file changed: %v", err)
			}
			valid := DefaultSettings()
			manager := &Manager{filePath: path, settings: valid}
			if err := manager.Save(settings); err == nil {
				t.Fatal("invalid save was silently rewritten")
			}
			current, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(current, original) || manager.Get() != valid {
				t.Fatalf("rejected save changed known state: %v", err)
			}
		})
	}
}

func TestConfigNullMissingAndAmbiguousFieldsPreserveTheSavedFile(t *testing.T) {
	for _, change := range []struct {
		name string
		from string
		to   string
	}{
		{name: "null boolean", from: `"enabled":true`, to: `"enabled":null`},
		{name: "missing boolean", from: `"enabled":true,`, to: ""},
		{name: "null section", from: `"power":{"preventSleepDuringSeed":true}`, to: `"power":null`},
		{name: "duplicate endpoint", from: `"webApiBaseUrl":"https://jointhectl.com"`, to: `"webApiBaseUrl":"https://invalid.example","webApiBaseUrl":"https://jointhectl.com"`},
	} {
		t.Run(change.name, func(t *testing.T) {
			path, original := writeTestSettings(t, DefaultSettings())
			modified := bytes.Replace(original, []byte(change.from), []byte(change.to), 1)
			if bytes.Equal(modified, original) {
				t.Fatal("test did not change its fixture")
			}
			if err := os.WriteFile(path, modified, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewManager(); err == nil {
				t.Fatal("unknown setting state became a guessed boolean or endpoint")
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, modified) {
				t.Fatalf("invalid fixture was overwritten: %v", err)
			}
		})
	}
}

func TestExplicitDoNothingDoesNotMigrateIntoLegacyShutdown(t *testing.T) {
	settings := DefaultSettings()
	updated, changed := normalizeSettings(settings, "shutdown")
	if changed || updated.Seed.DefaultPostSeedAction != "do_nothing" {
		t.Fatal("explicit current action was replaced by a deprecated shutdown")
	}
}

func TestNewManagerPreservesInvalidSettingsFile(t *testing.T) {
	for _, contents := range []string{"{invalid-json", "[]", "null"} {
		t.Run(contents, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", root)
			t.Setenv("APPDATA", root)
			appDir := filepath.Join(root, "citadel-sentry")
			if err := os.MkdirAll(appDir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(appDir, "config.json")
			original := []byte(contents)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewManager(); err == nil {
				t.Fatal("invalid settings were accepted instead of reporting an error")
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("invalid settings were replaced with defaults: %q, error %v", current, err)
			}
		})
	}
}

func TestNewManagerReportsUnreadableSettingsFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("APPDATA", root)
	path := filepath.Join(root, "citadel-sentry", "config.json")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(); err == nil {
		t.Fatal("failed read/write was hidden by a successful initialization")
	}
}

func TestNewManagerCreatesMissingSettingsFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("APPDATA", root)
	manager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if manager.Get() != DefaultSettings() {
		t.Fatal("new installation did not load the canonical defaults")
	}
	if _, err := os.Stat(filepath.Join(root, "citadel-sentry", "config.json")); err != nil {
		t.Fatalf("settings were not persisted: %v", err)
	}
}

func TestNormalizeSettingsMigratesLegacyPowerAction(t *testing.T) {
	settings := DefaultSettings()
	settings.Seed.DefaultPostSeedAction = "stay"

	updated, changed := normalizeSettings(settings, "shutdown")

	if !changed {
		t.Fatal("normalizeSettings() did not report a migration")
	}
	if updated.Seed.DefaultPostSeedAction != "shutdown" {
		t.Fatalf("default post-seed action = %q, want shutdown", updated.Seed.DefaultPostSeedAction)
	}
}

func TestNormalizeSettingsRemovesLegacyContinueNextAction(t *testing.T) {
	settings := DefaultSettings()
	settings.Seed.DefaultPostSeedAction = "continue_next"

	updated, changed := normalizeSettings(settings, "")

	if !changed {
		t.Fatal("normalizeSettings() did not report a migration")
	}
	if updated.Seed.DefaultPostSeedAction != "do_nothing" {
		t.Fatalf("default post-seed action = %q, want do_nothing", updated.Seed.DefaultPostSeedAction)
	}
}

func TestSaveDoesNotPersistRemovedSettings(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{filePath: filepath.Join(dir, "config.json"), settings: DefaultSettings()}

	if err := manager.Save(DefaultSettings()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(manager.filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	contents := string(data)
	for _, removed := range []string{"autoJoinSquadOnSeed", "squadExecutablePath", "powerActionAfterSeed"} {
		if strings.Contains(contents, removed) {
			t.Errorf("saved config contains removed setting %q", removed)
		}
	}
}

func TestConfigLoadsOlderVersionWithoutAutoUpdateCheck(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("APPDATA", root)
	directory := filepath.Join(root, "citadel-sentry")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	// Configuration JSON without autoUpdateCheck
	rawConfig := `{
		"general": {
			"launchOnStartup": false,
			"minimizeToTray": true,
			"closeToTray": true,
			"notificationsEnabled": true,
			"soundEnabled": true,
			"language": "ru"
		},
		"seed": {
			"defaultTargetPlayers": 50,
			"defaultMaxDurationMinutes": 60,
			"defaultPostSeedAction": "do_nothing"
		},
		"power": {
			"preventSleepDuringSeed": true
		},
		"potato": {
			"enabled": true,
			"fpsLimit": 1,
			"lowResolution": false,
			"lowCpuPriority": true,
			"autoRestoreOnExit": true
		},
		"webApiBaseUrl": "https://jointhectl.com"
	}`
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, []byte(rawConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() failed on older config: %v", err)
	}
	if !manager.Get().General.AutoUpdateCheck {
		t.Errorf("expected AutoUpdateCheck to default to true for older configs")
	}
}
