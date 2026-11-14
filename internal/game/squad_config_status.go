package game

import (
	"fmt"
	"math"
	"strconv"
)

func observedFpsCap(data []byte) (float64, error) {
	settings, duplicates := iniSettings(data)
	hasFpsCap := false
	var fpsCap float64
	for _, section := range []string{squadUserSettingsSection, "/script/engine.gameusersettings", "/script/squad.squadgameusersettings"} {
		id := section + "\x00frameratelimit"
		setting, exists := settings[id]
		if !exists {
			continue
		}
		value, err := strconv.ParseFloat(setting.value, 64)
		if duplicates[id] || err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf("current FrameRateLimit in [%s] is invalid or ambiguous", setting.section)
		}
		if hasFpsCap && value != fpsCap {
			return 0, fmt.Errorf("supported Squad sections contain conflicting FrameRateLimit values")
		}
		hasFpsCap = true
		fpsCap = value
	}
	if !hasFpsCap {
		return 0, fmt.Errorf("current FrameRateLimit was not found in a supported Squad section")
	}
	return fpsCap, nil
}
