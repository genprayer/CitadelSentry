package game

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"citadel-seeder/internal/logger"
	"citadel-seeder/internal/platform"
)

// SquadConfigStatus describes the current state of Squad graphics configuration.
type SquadConfigStatus struct {
	ConfigFound       bool    `json:"configFound"`
	ConfigPath        string  `json:"configPath"`
	HasBackup         bool    `json:"hasBackup"`
	HasPendingRestore bool    `json:"hasPendingRestore"`
	HasBaseline       bool    `json:"hasBaseline"`
	BackupPath        string  `json:"backupPath"`
	IsPotatoMode      bool    `json:"isPotatoMode"`
	CurrentFpsCap     float64 `json:"currentFpsCap"`
	ErrorMessage      string  `json:"errorMessage,omitempty"`
}

// BackupManifest stores metadata to verify backup integrity.
type BackupManifest struct {
	Version             int           `json:"version"`
	OriginalPath        string        `json:"originalPath"`
	Sha256              string        `json:"sha256"`
	Timestamp           time.Time     `json:"timestamp"`
	SizeBytes           int64         `json:"sizeBytes"`
	Active              bool          `json:"active"`
	CanKeepAfterSession bool          `json:"canKeepAfterSession,omitempty"`
	Files               []BackupEntry `json:"files,omitempty"`
}

// PatchedBackup records a historical patch file generated during a session.
type PatchedBackup struct {
	Name   string `json:"name"`
	Sha256 string `json:"sha256"`
}

// BackupEntry records one file changed for a potato session.
type BackupEntry struct {
	Path            string          `json:"path"`
	BackupName      string          `json:"backupName,omitempty"`
	PatchedName     string          `json:"patchedName,omitempty"`
	Sha256          string          `json:"sha256,omitempty"`
	SizeBytes       int64           `json:"sizeBytes"`
	PatchedSha256   string          `json:"patchedSha256"`
	Existed         bool            `json:"existed"`
	OriginalMode    *uint32         `json:"originalMode,omitempty"`
	PreviousPatches []PatchedBackup `json:"previousPatches,omitempty"`
}

// ConfigManager handles backup, modification (potato mode), and restoration of Squad configs.
type ConfigManager struct {
	mu             sync.RWMutex
	configPath     string
	secondaryPaths []string
	backupDir      string
	detectPaths    func() (string, []string)
}

// NewConfigManager initializes the manager targeting the standard system paths.
func NewConfigManager() (*ConfigManager, error) {
	backupDir, err := detectBackupDir()
	if err != nil {
		return nil, err
	}
	return newConfigManager(backupDir, detectSquadConfigPaths)
}

func newConfigManager(backupDir string, detectPaths func() (string, []string)) (*ConfigManager, error) {
	primary, secondaries := detectPaths()
	mgr := &ConfigManager{
		configPath:     primary,
		secondaryPaths: secondaries,
		backupDir:      backupDir,
		detectPaths:    detectPaths,
	}
	// An unfinished transaction owns its original path even if Squad has since
	// created a larger settings file in another candidate directory.
	if manifest, err := mgr.readManifestLocked(); err == nil && manifest.OriginalPath != "" && (manifest.Active || (manifest.Version == 0 && manifest.Sha256 != "")) {
		if err := mgr.validateEntryPathLocked(manifest.OriginalPath); err == nil {
			mgr.configPath = manifest.OriginalPath
		}
	}
	if err := mgr.migrateLegacyBackup(); err != nil {
		logger.Warn("Could not migrate legacy Squad backup", "error", err)
	}
	if !mgr.HasActiveBackup() {
		mgr.configPath = primary
	}
	_ = mgr.ensureInitialBaselineCapturedLocked()
	return mgr, nil
}

// NewConfigManagerWithPaths allows injecting custom paths (for unit tests).
func NewConfigManagerWithPaths(configPath, backupDir string) *ConfigManager {
	mgr := &ConfigManager{
		configPath: configPath,
		backupDir:  backupDir,
	}
	if err := mgr.migrateLegacyBackup(); err != nil {
		logger.Warn("Could not migrate legacy Squad backup", "error", err)
	}
	_ = mgr.ensureInitialBaselineCapturedLocked()
	return mgr
}

// ConfigPath returns the detected GameUserSettings.ini path.
func (m *ConfigManager) ConfigPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
}

// BackupPath returns the destination path of the backed up file.
func (m *ConfigManager) BackupPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return filepath.Join(m.backupDir, "GameUserSettings.ini.original")
}

func (m *ConfigManager) manifestPath() string {
	return filepath.Join(m.backupDir, "backup_manifest.json")
}

// HasActiveBackup protects unfinished or unverifiable recovery state from replacement.
func (m *ConfigManager) HasActiveBackup() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	manifest, err := m.readManifestLocked()
	if err == nil {
		err = m.verifyRecoveryIntegrityLocked(manifest)
	}
	return err != nil || manifest.Active
}

// ShouldRestoreAutomatically preserves recovery of interrupted sessions across restarts.
func (m *ConfigManager) ShouldRestoreAutomatically(isAutoRestoreEnabled bool) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	manifest, err := m.readManifestLocked()
	if err != nil {
		return false, err
	}
	if err := m.verifyRecoveryIntegrityLocked(manifest); err != nil {
		return false, err
	}
	return manifest.Active && (isAutoRestoreEnabled || !manifest.CanKeepAfterSession), nil
}

// MarkSessionFinished permits opt-out only after the owning worker finishes without failure.
func (m *ConfigManager) MarkSessionFinished() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest, err := m.readManifestLocked()
	if err != nil {
		return err
	}
	if err := m.verifyRecoveryIntegrityLocked(manifest); err != nil {
		return err
	}
	if !manifest.Active || manifest.CanKeepAfterSession {
		return nil
	}
	if manifest.Version != backupManifestVersion || manifest.OriginalPath != m.configPath {
		return fmt.Errorf("Squad backup manifest does not match the finished session")
	}
	manifest.CanKeepAfterSession = true
	return m.writeManifestLocked(manifest)
}

// BackupConfig saves a baseline copy without marking a potato session active.
func (m *ConfigManager) BackupConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshDetectedPathsLocked()
	return m.backupConfigLocked(false, false)
}

// ApplyPotatoConfig snapshots every affected file before changing settings.
func (m *ConfigManager) ApplyPotatoConfig(fpsLimit int, lowResolution bool, gpu ...platform.GPUInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshDetectedPathsLocked()
	return m.applyPotatoLocked(fpsLimit, lowResolution, gpu...)
}

// UpdateActivePotatoFps modifies the FrameRateLimit of an active Potato session
// without altering the original backup files.
func (m *ConfigManager) UpdateActivePotatoFps(fpsLimit int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateActiveFpsLocked(fpsLimit)
}

// SaveCurrentAsBackup refreshes the user's baseline after normal graphics changes.
func (m *ConfigManager) SaveCurrentAsBackup(force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshDetectedPathsLocked()
	return m.backupConfigLocked(true, force)
}

// RestoreOriginalConfig restores only an active, verified potato transaction.
func (m *ConfigManager) RestoreOriginalConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.restoreOriginalLocked(); err != nil {
		return err
	}
	m.refreshDetectedPathsLocked()
	return nil
}

// SetShowEmptyServers ensures the browser displays empty servers, enabling discovery of new seed targets.
func (m *ConfigManager) SetShowEmptyServers(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	val := "False"
	if enabled {
		val = "True"
	}

	var data []byte
	if _, err := os.Stat(m.configPath); err == nil {
		data, err = os.ReadFile(m.configPath)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	text := string(data)
	for _, section := range []string{"/script/squad.sqgameusersettings", "/script/engine.gameusersettings", "/script/squad.squadgameusersettings"} {
		text = setIniSectionKey(text, section, "bShowEmptyServers", val)
	}

	return os.WriteFile(m.configPath, []byte(text), 0644)
}

func (m *ConfigManager) refreshDetectedPathsLocked() {
	if m.detectPaths == nil {
		return
	}
	if manifest, err := m.readManifestLocked(); err != nil || manifest.Active {
		return
	}
	m.configPath, m.secondaryPaths = m.detectPaths()
}

// GetStatus returns the current on-disk configuration status.
func (m *ConfigManager) GetStatus() SquadConfigStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshDetectedPathsLocked()

	status := SquadConfigStatus{
		ConfigPath:  m.configPath,
		BackupPath:  filepath.Join(m.backupDir, "GameUserSettings.ini.original"),
		HasBaseline: m.HasInitialBaseline(),
	}
	var statusErrors []error
	if m.configPath != "" {
		if fi, err := os.Lstat(m.configPath); err == nil && fi.Mode().IsRegular() {
			status.ConfigFound = true
			data, readErr := os.ReadFile(m.configPath)
			if readErr != nil {
				statusErrors = append(statusErrors, fmt.Errorf("read current Squad settings: %w", readErr))
			} else {
				status.IsPotatoMode = looksLikePotatoConfig(data)
				status.CurrentFpsCap, readErr = observedFpsCap(data)
				if readErr != nil {
					statusErrors = append(statusErrors, readErr)
				}
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			statusErrors = append(statusErrors, fmt.Errorf("inspect current Squad settings: %w", err))
		} else if err == nil {
			statusErrors = append(statusErrors, errors.New("current Squad settings path is not a regular file"))
		}
	}
	if info, err := os.Lstat(status.BackupPath); err == nil && info.Mode().IsRegular() {
		status.HasBackup = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		statusErrors = append(statusErrors, fmt.Errorf("inspect Squad baseline backup: %w", err))
	} else if err == nil {
		statusErrors = append(statusErrors, errors.New("Squad baseline backup is not a regular file"))
	}
	if manifest, err := m.readManifestLocked(); err == nil {
		status.HasPendingRestore = manifest.Active
		status.IsPotatoMode = status.IsPotatoMode || manifest.Active
		if err := m.verifyBackupIntegrityLocked(manifest); err != nil {
			status.HasBackup = false
			statusErrors = append(statusErrors, err)
		}
	} else {
		status.HasBackup = false
		statusErrors = append(statusErrors, err)
	}
	if err := errors.Join(statusErrors...); err != nil {
		status.ErrorMessage = err.Error()
	}
	return status
}

// setIniSectionKey updates or inserts a key=val pair specifically inside the named INI section.
// If the section doesn't exist, it is created, preserving original CRLF/LF line endings.
func setIniSectionKey(text, section, key, val string) string {
	lineEnding := "\n"
	if strings.Contains(text, "\r\n") {
		lineEnding = "\r\n"
	}

	sectionHeader := "[" + section + "]"
	reSection := regexp.MustCompile(fmt.Sprintf(`(?im)^[ \t]*\[%s\][ \t]*\r?$`, regexp.QuoteMeta(section)))
	sectionMatches := reSection.FindAllStringIndex(text, -1)
	if len(sectionMatches) == 0 {
		trimmed := strings.TrimRight(text, "\r\n")
		if trimmed == "" {
			return sectionHeader + lineEnding + key + "=" + val + lineEnding
		}
		return trimmed + lineEnding + lineEnding + sectionHeader + lineEnding + key + "=" + val + lineEnding
	}
	secIdx := sectionMatches[len(sectionMatches)-1][0]
	reKey := regexp.MustCompile(fmt.Sprintf(`(?im)^[ \t]*%s[ \t]*=[^\r\n]*`, regexp.QuoteMeta(key)))
	reNextSec := regexp.MustCompile(`(?m)^[ \t]*\[[^\r\n]+\][ \t]*\r?$`)
	for _, match := range sectionMatches {
		end := len(text)
		if next := reNextSec.FindStringIndex(text[match[1]:]); next != nil {
			end = match[1] + next[0]
		}
		if reKey.MatchString(text[match[1]:end]) {
			secIdx = match[0]
			break
		}
	}

	headerLineEnd := strings.Index(text[secIdx:], "\n")
	if headerLineEnd == -1 {
		return text + lineEnding + key + "=" + val + lineEnding
	}
	contentStart := secIdx + headerLineEnd + 1

	nextSecIdx := -1
	subText := text[contentStart:]
	loc := reNextSec.FindStringIndex(subText)
	if loc != nil {
		nextSecIdx = contentStart + loc[0]
	}

	var sectionBody, remainder string
	if nextSecIdx != -1 {
		sectionBody = text[contentStart:nextSecIdx]
		remainder = text[nextSecIdx:]
	} else {
		sectionBody = text[contentStart:]
		remainder = ""
	}

	if reKey.MatchString(sectionBody) {
		sectionBody = reKey.ReplaceAllStringFunc(sectionBody, func(string) string { return key + "=" + val })
	} else {
		sectionBody = fmt.Sprintf("%s=%s%s", key, val, lineEnding) + sectionBody
	}

	return text[:contentStart] + sectionBody + remainder
}

// patchPotatoIni changes keys already present in supported game sections and
// injects potato settings into the active user and scalability sections.
func patchPotatoIni(original []byte, fpsLimit int, lowResolution bool, gpu ...platform.GPUInfo) []byte {
	text := string(original)
	settings, _ := iniSettings(original)
	changes := potatoSettingValues(fpsLimit, lowResolution, gpu...)

	// If user already had TSR (UpscaleMode=4) enabled, preserve TSR with optimized performance values
	hasTSR := false
	for id, setting := range settings {
		if strings.HasSuffix(id, "\x00upscalemode") && setting.value == "4" {
			hasTSR = true
			break
		}
	}
	if hasTSR {
		for _, s := range []string{squadUserSettingsSection, "/script/engine.gameusersettings", "/script/squad.squadgameusersettings"} {
			if sec, ok := changes[s]; ok {
				sec["upscalemode"] = "4"
				delete(sec, "dlssmode")
				sec["tsrsamplecount"] = "(Value=8)"
				sec["tsrflickeringperiod"] = "(Value=3)"
			}
		}
	}

	for _, setting := range settings {
		if value, shouldChange := changes[strings.ToLower(setting.section)][strings.ToLower(setting.key)]; shouldChange {
			text = setIniSectionKey(text, setting.section, setting.key, value)
		}
	}
	if lowResolution {
		hasResX := false
		for id := range settings {
			if strings.HasSuffix(id, "\x00resolutionsizex") {
				hasResX = true
				break
			}
		}
		if !hasResX {
			targetSection := "/Script/Engine.GameUserSettings"
			for _, s := range []string{"/Script/Engine.GameUserSettings", "/Script/Squad.SquadGameUserSettings", squadUserSettingsSection} {
				if _, exists := settings[strings.ToLower(s)+"\x00frameratelimit"]; exists {
					targetSection = s
					break
				}
			}
			text = setIniSectionKey(text, targetSection, "ResolutionSizeX", strconv.Itoa(potatoResolutionWidth))
			text = setIniSectionKey(text, targetSection, "ResolutionSizeY", strconv.Itoa(potatoResolutionHeight))
			text = setIniSectionKey(text, targetSection, "FullscreenMode", strconv.Itoa(potatoWindowMode))
			text = setIniSectionKey(text, targetSection, "WindowMode", strconv.Itoa(potatoWindowMode))
			text = setIniSectionKey(text, targetSection, "DesiredScreenWidth", strconv.Itoa(potatoResolutionWidth))
			text = setIniSectionKey(text, targetSection, "DesiredScreenHeight", strconv.Itoa(potatoResolutionHeight))
			text = setIniSectionKey(text, targetSection, "LastUserConfirmedResolutionSizeX", strconv.Itoa(potatoResolutionWidth))
			text = setIniSectionKey(text, targetSection, "LastUserConfirmedResolutionSizeY", strconv.Itoa(potatoResolutionHeight))
		}
	}
	return []byte(text)
}

// detectSquadConfigPaths searches candidate directories prioritizing modern Squad (WindowsClient),
// returning the primary active config path and all existing candidate paths.
func detectSquadConfigPaths() (primary string, allExisting []string) {
	return selectSquadConfigPaths(getSquadConfigCandidates())
}

func selectSquadConfigPaths(candidates []string) (primary string, allExisting []string) {

	// 1. Gather all existing candidate files
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			allExisting = append(allExisting, c)
		}
	}

	// 2. First pass: look for an existing file with genuine user settings (size >= 1000 bytes)
	for _, c := range allExisting {
		if fi, err := os.Stat(c); err == nil && fi.Size() >= 1000 {
			primary = c
			break
		}
	}

	// 3. Second pass: if no file >= 1000 bytes, look for any existing non-potato file
	if primary == "" {
		for _, c := range allExisting {
			if data, rErr := os.ReadFile(c); rErr == nil {
				if !looksLikePotatoConfig(data) {
					primary = c
					break
				}
			}
		}
	}

	// 4. Third pass: if all existing files are small potato dummies, pick first existing candidate
	if primary == "" && len(allExisting) > 0 {
		primary = allExisting[0]
	}

	// If a primary was found from existing files, return it
	if primary != "" {
		return primary, allExisting
	}

	// 2. If no files exist yet, check which parent directory exists (WindowsClient prioritized)
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return c, allExisting
		}
	}

	// 3. Fallback: modern Windows default
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "SquadGame", "Saved", "Config", "WindowsClient", "GameUserSettings.ini"), allExisting
	}

	if len(candidates) > 0 {
		return candidates[0], allExisting
	}

	return "", allExisting
}

func getSquadConfigCandidates() []string {
	var baseDirs []string

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		baseDirs = append(baseDirs,
			filepath.Join(localAppData, "SquadGame", "Saved", "Config"),
			filepath.Join(localAppData, "Squad", "Saved", "Config"),
		)
	}

	if home, err := os.UserHomeDir(); err == nil {
		baseDirs = append(baseDirs, steamSquadConfigBases(home, os.Getenv("XDG_DATA_HOME"))...)
	}

	// Priority order: WindowsClient (UE 4.27+ modern Squad) > Windows (UE5) > WindowsNoEditor (legacy UE4)
	subDirs := []string{"WindowsClient", "Windows", "WindowsNoEditor"}
	var candidates []string
	for _, base := range baseDirs {
		for _, sub := range subDirs {
			candidates = append(candidates, filepath.Join(base, sub, "GameUserSettings.ini"))
		}
	}
	return candidates
}

func detectSquadConfigPath() string {
	p, _ := detectSquadConfigPaths()
	return p
}

func detectBackupDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine Squad backup directory: %w", err)
	}
	return filepath.Join(configDir, "citadel-sentry", "squad_backup"), nil
}

// CheckMatchesPotatoSettings checks whether the on-disk Squad configuration matches the requested potato preset.
func (m *ConfigManager) CheckMatchesPotatoSettings(fpsLimit int, lowResolution bool) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.configPath == "" {
		return false, errors.New("squad config path not found")
	}
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return false, fmt.Errorf("read squad config: %w", err)
	}
	return CheckMatchesPotatoSettings(data, fpsLimit, lowResolution)
}

