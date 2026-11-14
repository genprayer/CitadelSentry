package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"citadel-seeder/internal/fileutil"
)

const maxSettingsFileBytes = 1 << 20

// MaxSeedDurationMinutes is the largest whole-minute duration representable by Go.
const MaxSeedDurationMinutes = int((1<<63 - 1) / time.Minute)

// ValidateSettings checks explicit choices without replacing unknown values with defaults.
func ValidateSettings(settings AppSettings) error {
	endpoint, err := url.Parse(settings.WebApiBaseUrl)
	if err != nil || endpoint.Hostname() == "" || (!strings.EqualFold(endpoint.Scheme, "https") && !strings.EqualFold(endpoint.Scheme, "http")) ||
		strings.ContainsAny(settings.WebApiBaseUrl, "?#") || settings.WebApiBaseUrl != strings.TrimSpace(settings.WebApiBaseUrl) {
		return fmt.Errorf("webApiBaseUrl must be an absolute HTTP(S) base URL without a query or fragment")
	}
	if settings.Seed.DefaultTargetPlayers <= 0 || uint64(settings.Seed.DefaultTargetPlayers) > (1<<53-1) {
		return fmt.Errorf("defaultTargetPlayers must be a positive JavaScript-safe integer")
	}
	if settings.Seed.DefaultMaxDurationMinutes <= 0 || settings.Seed.DefaultMaxDurationMinutes > MaxSeedDurationMinutes {
		return fmt.Errorf("defaultMaxDurationMinutes must fit a positive duration")
	}
	switch settings.Seed.DefaultPostSeedAction {
	case "do_nothing", "close_squad", "sleep", "shutdown":
	default:
		return fmt.Errorf("unsupported defaultPostSeedAction: %q", settings.Seed.DefaultPostSeedAction)
	}
	if settings.Potato.FpsLimit <= 0 || settings.Potato.FpsLimit > 60 {
		return fmt.Errorf("fpsLimit must be between 1 and 60")
	}
	if settings.General.Language != "ru" && settings.General.Language != "en" {
		return fmt.Errorf("unsupported language: %q", settings.General.Language)
	}
	return nil
}

func decodeSavedSettings(path string) (AppSettings, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return AppSettings{}, "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSettingsFileBytes+1))
	if err != nil {
		return AppSettings{}, "", err
	}
	if len(data) > maxSettingsFileBytes {
		return AppSettings{}, "", fmt.Errorf("application settings file exceeds its size limit")
	}
	var fields map[string]json.RawMessage
	if err := fileutil.DecodeJSONStrict(data, &fields); err != nil {
		return AppSettings{}, "", err
	}
	if fields == nil {
		return AppSettings{}, "", fmt.Errorf("application settings must be a JSON object")
	}
	legacyPowerAction := ""
	if power, exists := fields["power"]; exists {
		var powerFields map[string]json.RawMessage
		if err := json.Unmarshal(power, &powerFields); err != nil {
			return AppSettings{}, "", err
		}
		if action, exists := powerFields["powerActionAfterSeed"]; exists {
			if bytes.Equal(bytes.TrimSpace(action), []byte("null")) {
				return AppSettings{}, "", fmt.Errorf("legacy power action cannot be null")
			}
			if err := json.Unmarshal(action, &legacyPowerAction); err != nil {
				return AppSettings{}, "", err
			}
		}
	}
	if err := requireSettingsFields(fields, reflect.TypeOf(AppSettings{}), "", legacyPowerAction); err != nil {
		return AppSettings{}, "", err
	}
	var settings AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return AppSettings{}, "", err
	}
	return settings, legacyPowerAction, nil
}

func requireSettingsFields(fields map[string]json.RawMessage, kind reflect.Type, prefix string, legacyPowerAction string) error {
	for index := 0; index < kind.NumField(); index++ {
		field := kind.Field(index)
		key := field.Tag.Get("json")
		name := prefix + key
		value, exists := fields[key]
		if !exists {
			if name == "seed.defaultPostSeedAction" && (legacyPowerAction == "sleep" || legacyPowerAction == "shutdown") {
				continue
			}
			return fmt.Errorf("saved application setting %s is missing", name)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("saved application setting %s is null", name)
		}
		if field.Type.Kind() == reflect.Struct {
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(value, &nested); err != nil {
				return fmt.Errorf("decode %s: %w", name, err)
			}
			if err := requireSettingsFields(nested, field.Type, name+".", legacyPowerAction); err != nil {
				return err
			}
		}
	}
	return nil
}
