package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"citadel-seeder/internal/platform"
)

const (
	squadUserSettingsSection = "/script/squad.sqgameusersettings"
	potatoResolutionWidth    = 640
	potatoResolutionHeight   = 480
	potatoWindowMode         = 2
	potatoResolutionQuality  = 25
	potatoMenuFpsLimit       = 60
)

// potatoSettingValues describes the selected preset, never missing original values.
func potatoSettingValues(fpsLimit int, lowResolution bool, gpu ...platform.GPUInfo) map[string]map[string]string {
	menuFps := potatoMenuFpsLimit
	if fpsLimit > menuFps {
		menuFps = fpsLimit
	}
	userValues := map[string]string{
		"frameratelimit":               fmt.Sprintf("%.6f", float64(fpsLimit)),
		"menuframeratelimit":           fmt.Sprintf("%.6f", float64(menuFps)),
		"bshowemptyservers":            "True",
		"busevsync":                    "False",
		"buncaptexturepoolsize":        "False",
		"texturestreampoolsizestorage": "(Value=0)",
		"materialquality":              "(Value=0)",
		"oceanquality":                 "(Value=0)",
		"wakesim":                      "(Value=0)",
		"skeletalmeshlodbias":          "(Value=1)",
		"foliageminlod":                "(Value=1)",
		"scoperesolutionscale":         "(Value=0)",
		"scopeupdaterate":              "(Value=30)",
	}

	var g platform.GPUInfo
	if len(gpu) > 0 {
		g = gpu[0]
	} else {
		g = platform.DetectGPU()
	}

	if g.Vendor == platform.GPUVendorNVIDIA && g.IsRTX {
		userValues["upscalemode"] = "2"
		userValues["dlssmode"] = "UltraPerformance"
	} else {
		userValues["upscalemode"] = "1"
		userValues["fsrquality"] = "(Value=4)"
		if g.Vendor == platform.GPUVendorAMD {
			userValues["selectedframegeneration"] = "AMDFG"
		}
	}

	if lowResolution {
		for _, key := range []string{"resolutionsizex", "lastuserconfirmedresolutionsizex", "desiredscreenwidth", "lastuserconfirmeddesiredscreenwidth"} {
			userValues[key] = strconv.Itoa(potatoResolutionWidth)
		}
		for _, key := range []string{"resolutionsizey", "lastuserconfirmedresolutionsizey", "desiredscreenheight", "lastuserconfirmeddesiredscreenheight"} {
			userValues[key] = strconv.Itoa(potatoResolutionHeight)
		}
		for _, key := range []string{"windowmode", "fullscreenmode", "lastconfirmedfullscreenmode", "preferredfullscreenmode"} {
			userValues[key] = strconv.Itoa(potatoWindowMode)
		}
		userValues["busedesiredscreenheight"] = "False"
	}
	return map[string]map[string]string{
		squadUserSettingsSection:              userValues,
		"/script/engine.gameusersettings":     userValues,
		"/script/squad.squadgameusersettings": userValues,
		"scalabilitygroups": {
			"sg.resolutionquality":         fmt.Sprintf("%.6f", float64(potatoResolutionQuality)),
			"sg.viewdistancequality":       "0",
			"sg.antialiasingquality":       "0",
			"sg.shadowquality":             "0",
			"sg.postprocessquality":        "0",
			"sg.texturequality":            "0",
			"sg.effectsquality":            "0",
			"sg.foliagequality":            "0",
			"sg.shadingquality":            "0",
			"sg.landscapequality":          "0",
			"sg.globalilluminationquality": "0",
			"sg.reflectionquality":         "0",
		},
	}
}

// validatePotatoSettings refuses unknown or ambiguous game settings before writes.
func validatePotatoSettings(original []byte, fpsLimit int, lowResolution bool, gpu ...platform.GPUInfo) error {
	settings, duplicates := iniSettings(original)
	changes := potatoSettingValues(fpsLimit, lowResolution, gpu...)
	hasFpsSetting := false
	for id, setting := range settings {
		section := strings.ToLower(setting.section)
		key := strings.ToLower(setting.key)
		if _, shouldChange := changes[section][key]; !shouldChange {
			continue
		}
		if duplicates[id] {
			return fmt.Errorf("repeated Potato Mode key %s in [%s]", setting.key, setting.section)
		}
		if strings.HasPrefix(key, "buse") || strings.HasPrefix(key, "buncap") {
			if _, err := strconv.ParseBool(setting.value); err != nil {
				return fmt.Errorf("unsupported %s value in [%s]", setting.key, setting.section)
			}
		} else if strings.HasPrefix(setting.value, "(Value=") && strings.HasSuffix(setting.value, ")") {
			inner := strings.TrimPrefix(strings.TrimSuffix(setting.value, ")"), "(Value=")
			if _, err := strconv.ParseFloat(inner, 64); err != nil {
				return fmt.Errorf("unsupported %s value in [%s]", setting.key, setting.section)
			}
		} else if key == "dlssmode" || key == "selectedframegeneration" {
			if len(setting.value) == 0 {
				return fmt.Errorf("empty %s value in [%s]", setting.key, setting.section)
			}
		} else if err := validateNumericSetting(setting); err != nil {
			return err
		}
		if key == "frameratelimit" {
			hasFpsSetting = true
		}
	}
	if !hasFpsSetting {
		return fmt.Errorf("FrameRateLimit not found in a supported Squad user settings section")
	}
	return nil
}

func validateNumericSetting(setting iniSetting) error {
	value, err := strconv.ParseFloat(setting.value, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("unsupported %s value in [%s]", setting.key, setting.section)
	}
	return nil
}

// CheckMatchesPotatoSettings reports whether the given config data already matches
// the requested potato mode preset (FPS limit and resolution settings).
func CheckMatchesPotatoSettings(data []byte, fpsLimit int, lowResolution bool) (bool, error) {
	settings, _ := iniSettings(data)
	if len(settings) == 0 {
		return false, errors.New("empty or invalid ini settings")
	}
	expectedFps := float64(fpsLimit)

	foundFpsMatch := false
	for _, section := range []string{squadUserSettingsSection, "/script/engine.gameusersettings", "/script/squad.squadgameusersettings"} {
		if fpsSetting, exists := settings[section+"\x00frameratelimit"]; exists {
			val, err := strconv.ParseFloat(fpsSetting.value, 64)
			if err == nil && math.Abs(val-expectedFps) < 0.1 {
				foundFpsMatch = true
				break
			}
		}
	}
	if !foundFpsMatch {
		return false, nil
	}

	if lowResolution {
		foundResMatch := false
		for _, section := range []string{squadUserSettingsSection, "/script/engine.gameusersettings", "/script/squad.squadgameusersettings"} {
			width, hasW := settings[section+"\x00resolutionsizex"]
			height, hasH := settings[section+"\x00resolutionsizey"]
			if hasW && hasH && width.value == strconv.Itoa(potatoResolutionWidth) && height.value == strconv.Itoa(potatoResolutionHeight) {
				foundResMatch = true
				break
			}
		}
		if !foundResMatch {
			return false, nil
		}
	}

	return true, nil
}

