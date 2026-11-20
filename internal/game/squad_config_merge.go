package game

import (
	"fmt"
	"strconv"
	"strings"
)

type iniSetting struct {
	section string
	key     string
	value   string
}

func iniSettings(data []byte) (map[string]iniSetting, map[string]bool) {
	result := make(map[string]iniSetting)
	duplicates := make(map[string]bool)
	section := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			continue
		}
		if section == "" || trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		index := strings.IndexByte(line, '=')
		if index < 0 {
			continue
		}
		key := strings.TrimSpace(line[:index])
		if key == "" {
			continue
		}
		id := strings.ToLower(section) + "\x00" + strings.ToLower(key)
		if _, exists := result[id]; exists {
			duplicates[id] = true
		}
		result[id] = iniSetting{section: section, key: key, value: strings.TrimSpace(line[index+1:])}
	}
	return result, duplicates
}

func equalIniValue(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if strings.EqualFold(left, right) {
		return true
	}
	leftNumber, leftErr := strconv.ParseFloat(left, 64)
	rightNumber, rightErr := strconv.ParseFloat(right, 64)
	return leftErr == nil && rightErr == nil && leftNumber == rightNumber
}

// mergePotatoIni restores recorded keys missing or still matching Sentry's patch
// (including intermediate older patches from this session).
// Changes made by Squad or the player to other keys remain in place.
func mergePotatoIni(original, patched, current []byte, olderPatches ...[]byte) ([]byte, error) {
	originalSettings, originalDuplicates := iniSettings(original)
	patchedSettings, patchedDuplicates := iniSettings(patched)
	currentSettings, currentDuplicates := iniSettings(current)
	result := string(current)
	for id, patchedSetting := range patchedSettings {
		originalSetting, hadOriginal := originalSettings[id]
		if hadOriginal && equalIniValue(originalSetting.value, patchedSetting.value) {
			continue
		}
		if originalDuplicates[id] || patchedDuplicates[id] || currentDuplicates[id] {
			return nil, fmt.Errorf("repeated Potato Mode key %s in [%s]", patchedSetting.key, patchedSetting.section)
		}
		currentSetting, exists := currentSettings[id]
		if !exists {
			if hadOriginal {
				result = setIniSectionKey(result, originalSetting.section, originalSetting.key, originalSetting.value)
			}
			continue
		}
		matchesPatch := equalIniValue(currentSetting.value, patchedSetting.value)
		if !matchesPatch {
			for _, older := range olderPatches {
				olderSettings, _ := iniSettings(older)
				if oldSetting, exists := olderSettings[id]; exists && equalIniValue(currentSetting.value, oldSetting.value) {
					matchesPatch = true
					break
				}
			}
		}
		if !matchesPatch {
			continue
		}
		if hadOriginal {
			result = setIniSectionKey(result, currentSetting.section, currentSetting.key, originalSetting.value)
		} else {
			result = removeIniSectionKey(result, currentSetting.section, currentSetting.key)
		}
	}
	return []byte(result), nil
}

func removeIniSectionKey(text, section, key string) string {
	lines := strings.SplitAfter(text, "\n")
	currentSection := ""
	var result strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
		if strings.EqualFold(currentSection, section) {
			if index := strings.IndexByte(line, '='); index >= 0 && strings.EqualFold(strings.TrimSpace(line[:index]), key) {
				continue
			}
		}
		result.WriteString(line)
	}
	return result.String()
}
