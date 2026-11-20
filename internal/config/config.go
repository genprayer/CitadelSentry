package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"citadel-seeder/internal/fileutil"
)

// GeneralSettings holds system lifecycle and notification preferences.
type GeneralSettings struct {
	LaunchOnStartup      bool   `json:"launchOnStartup"`
	MinimizeToTray       bool   `json:"minimizeToTray"`
	CloseToTray          bool   `json:"closeToTray"`
	NotificationsEnabled bool   `json:"notificationsEnabled"`
	SoundEnabled         bool   `json:"soundEnabled"`
	Language             string `json:"language"`
	AutoUpdateCheck      bool   `json:"autoUpdateCheck"`
	TelemetryOptIn       bool   `json:"telemetryOptIn"`
	TelemetryPromptShown bool   `json:"telemetryPromptShown"`
}

// SeedSettings defines seeding engine operational thresholds and preferences.
type SeedSettings struct {
	DefaultTargetPlayers      int    `json:"defaultTargetPlayers"`
	DefaultMaxDurationMinutes int    `json:"defaultMaxDurationMinutes"`
	DefaultPostSeedAction     string `json:"defaultPostSeedAction"`
}

// PowerSettings defines OS energy saving and sleep inhibition behavior.
type PowerSettings struct {
	PreventSleepDuringSeed bool `json:"preventSleepDuringSeed"`
}

// PotatoSettings defines low-power graphics and CPU throttling for background seeding.
type PotatoSettings struct {
	Enabled           bool `json:"enabled"`
	FpsLimit          int  `json:"fpsLimit"`
	LowResolution     bool `json:"lowResolution"`
	LowCpuPriority    bool `json:"lowCpuPriority"`
	AutoRestoreOnExit bool `json:"autoRestoreOnExit"`
}

// AppSettings represents the root configuration saved to disk.
type AppSettings struct {
	General       GeneralSettings `json:"general"`
	Seed          SeedSettings    `json:"seed"`
	Power         PowerSettings   `json:"power"`
	Potato        PotatoSettings  `json:"potato"`
	WebApiBaseUrl string          `json:"webApiBaseUrl"`
}

// DefaultSettings returns the canonical production defaults.
func DefaultSettings() AppSettings {
	return AppSettings{
		General: GeneralSettings{
			LaunchOnStartup:      false,
			MinimizeToTray:       true,
			CloseToTray:          true,
			NotificationsEnabled: true,
			SoundEnabled:         true,
			Language:             "ru",
			AutoUpdateCheck:      true,
			TelemetryOptIn:       false,
			TelemetryPromptShown: false,
		},
		Seed: SeedSettings{
			DefaultTargetPlayers:      50,
			DefaultMaxDurationMinutes: 60,
			DefaultPostSeedAction:     "do_nothing",
		},
		Power: PowerSettings{
			PreventSleepDuringSeed: true,
		},
		Potato: PotatoSettings{
			Enabled:           true,
			FpsLimit:          1,
			LowResolution:     false,
			LowCpuPriority:    true,
			AutoRestoreOnExit: true,
		},
		WebApiBaseUrl: "https://jointhectl.com",
	}
}

// Manager manages reading and persisting settings safely.
type Manager struct {
	mu       sync.RWMutex
	filePath string
	settings AppSettings
}

// StateDirectory resolves the application's actual user configuration directory.
func StateDirectory() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config directory: %w", err)
	}
	if !filepath.IsAbs(configDir) {
		return "", fmt.Errorf("user config directory must be absolute")
	}
	return filepath.Join(configDir, "citadel-sentry"), nil
}

// NewManager initializes configuration from disk or writes defaults.
func NewManager() (*Manager, error) {
	return NewManagerWithDefaults(DefaultSettings())
}

// NewManagerWithDefaults uses explicit initial choices only for a genuinely missing file.
func NewManagerWithDefaults(initial AppSettings) (*Manager, error) {
	if err := ValidateSettings(initial); err != nil {
		return nil, fmt.Errorf("validate initial settings: %w", err)
	}
	appDir, err := StateDirectory()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}
	if err := os.Chmod(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to secure config dir: %w", err)
	}

	filePath := filepath.Join(appDir, "config.json")
	mgr := &Manager{
		filePath: filePath,
		settings: initial,
	}

	if err := mgr.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load application settings: %w", err)
		}
		data, err := json.MarshalIndent(mgr.settings, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode initial settings: %w", err)
		}
		if err := fileutil.CreateFileAtomic(filePath, data, 0o600); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create initial settings: %w", err)
		}
		// Another instance may have created the file; load its actual contents.
		if err := mgr.Load(); err != nil {
			return nil, fmt.Errorf("load initial settings: %w", err)
		}
	}

	return mgr, nil
}

// Load reads settings from disk.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	loaded, legacyPowerAction, err := decodeSavedSettings(m.filePath)
	if err != nil {
		return err
	}

	settings, migrated := normalizeSettings(loaded, legacyPowerAction)
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	if migrated {
		updated, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return fmt.Errorf("encode migrated settings: %w", err)
		}
		if err := fileutil.WriteFileAtomic(m.filePath, updated, 0o600); err != nil {
			return fmt.Errorf("persist migrated settings: %w", err)
		}
	}

	m.settings = settings
	return nil
}

// Get returns a copy of the current settings.
func (m *Manager) Get() AppSettings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings
}

// Save writes new settings to disk.
func (m *Manager) Save(s AppSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := ValidateSettings(s); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := fileutil.WriteFileAtomic(m.filePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	m.settings = s
	return nil
}

func normalizeSettings(settings AppSettings, legacyPowerAction string) (AppSettings, bool) {
	changed := false

	// Older releases exposed the post-seed power action under settings.power.
	// Migrate that value once into the canonical seed action so existing users
	// retain their choice without keeping two competing configuration fields.
	if settings.Seed.DefaultPostSeedAction == "" ||
		settings.Seed.DefaultPostSeedAction == "stay" {
		switch legacyPowerAction {
		case "sleep", "shutdown":
			settings.Seed.DefaultPostSeedAction = legacyPowerAction
			changed = true
		}
	}

	switch settings.Seed.DefaultPostSeedAction {
	case "stay", "continue_next":
		settings.Seed.DefaultPostSeedAction = "do_nothing"
		changed = true
	case "close_game", "exit_app":
		settings.Seed.DefaultPostSeedAction = "close_squad"
		changed = true
	}
	return settings, changed
}
