package game

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"citadel-seeder/internal/fileutil"
	"citadel-seeder/internal/logger"
	"citadel-seeder/internal/platform"
)

const backupManifestVersion = 2

type configPatch struct {
	entry    BackupEntry
	original []byte
	patched  []byte
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *ConfigManager) readManifestLocked() (BackupManifest, error) {
	info, err := os.Lstat(m.manifestPath())
	if errors.Is(err, os.ErrNotExist) {
		entries, inspectErr := os.ReadDir(m.backupDir)
		if inspectErr != nil && !errors.Is(inspectErr, os.ErrNotExist) {
			return BackupManifest{}, fmt.Errorf("inspect Squad backup records: %w", inspectErr)
		}
		for _, entry := range entries {
			name := entry.Name()
			isSessionRecord := strings.HasPrefix(name, "session-") && (strings.HasSuffix(name, ".original") || strings.HasSuffix(name, ".patched"))
			if name == "GameUserSettings.ini.original" || isSessionRecord {
				return BackupManifest{}, errors.New("Squad backup records exist without their manifest; refusing to replace the saved originals")
			}
		}
		return BackupManifest{}, nil
	}
	if err != nil {
		return BackupManifest{}, fmt.Errorf("read Squad backup manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return BackupManifest{}, errors.New("Squad backup manifest is not a regular file")
	}
	if info.Size() > maxSquadManifestBytes {
		return BackupManifest{}, fmt.Errorf("Squad backup manifest exceeds its size limit")
	}
	file, err := os.Open(m.manifestPath())
	if err != nil {
		return BackupManifest{}, fmt.Errorf("open Squad backup manifest: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return BackupManifest{}, errors.New("Squad backup manifest changed before reading")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSquadManifestBytes+1))
	if err != nil {
		return BackupManifest{}, fmt.Errorf("read Squad backup manifest: %w", err)
	}
	if len(data) > maxSquadManifestBytes {
		return BackupManifest{}, fmt.Errorf("Squad backup manifest exceeds its size limit")
	}
	manifest, err := decodeSquadManifest(data)
	if err != nil {
		return BackupManifest{}, fmt.Errorf("parse Squad backup manifest: %w", err)
	}
	return manifest, nil
}

// migrateLegacyBackup adopts an old active backup only when its recorded bytes verify.
func (m *ConfigManager) migrateLegacyBackup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest, err := m.readManifestLocked()
	if err != nil || manifest.Version != 0 || manifest.OriginalPath != m.configPath || manifest.Sha256 == "" {
		return err
	}
	backup, err := os.ReadFile(filepath.Join(m.backupDir, "GameUserSettings.ini.original"))
	if err != nil || int64(len(backup)) != manifest.SizeBytes || checksum(backup) != manifest.Sha256 {
		return fmt.Errorf("legacy Squad backup checksum or size is invalid")
	}
	current, err := os.ReadFile(m.configPath)
	if err != nil || !looksLikePotatoConfig(current) || checksum(current) == manifest.Sha256 {
		return nil
	}
	manifest.Version = backupManifestVersion
	manifest.Active = true
	manifest.Files = []BackupEntry{{
		Path:          m.configPath,
		BackupName:    "GameUserSettings.ini.original",
		Sha256:        manifest.Sha256,
		SizeBytes:     manifest.SizeBytes,
		PatchedSha256: checksum(current),
		Existed:       true,
	}}
	return m.writeManifestLocked(manifest)
}

func (m *ConfigManager) archiveLegacyBackupLocked(manifest BackupManifest) error {
	if manifest.Version != 0 || manifest.OriginalPath == "" {
		return nil
	}
	archiveDir, err := os.MkdirTemp(m.backupDir, "legacy-")
	if err != nil {
		return fmt.Errorf("archive legacy Squad backup: %w", err)
	}
	for _, name := range []string{"GameUserSettings.ini.original", "backup_manifest.json"} {
		path := filepath.Join(m.backupDir, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("legacy %s is not a readable regular file: %w", name, errors.Join(err, errors.New("unsupported backup record")))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read legacy %s: %w", name, err)
		}
		if err := fileutil.WriteFileAtomic(filepath.Join(archiveDir, name), data, 0o600); err != nil {
			return fmt.Errorf("archive legacy %s: %w", name, err)
		}
	}
	return nil
}

func (m *ConfigManager) writeManifestLocked(manifest BackupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Squad backup manifest: %w", err)
	}
	if len(data) > maxSquadManifestBytes {
		return fmt.Errorf("Squad backup manifest exceeds its size limit")
	}
	if _, err := decodeSquadManifest(data); err != nil {
		return fmt.Errorf("validate Squad backup manifest before save: %w", err)
	}
	if err := fileutil.WriteFileAtomic(m.manifestPath(), data, 0o600); err != nil {
		return fmt.Errorf("write Squad backup manifest: %w", err)
	}
	return nil
}

func (m *ConfigManager) backupConfigLocked(refresh bool, force bool) error {
	if m.configPath == "" {
		return errors.New("Squad GameUserSettings.ini path not detected")
	}
	manifest, err := m.readManifestLocked()
	if err != nil {
		return err
	}
	if err := m.verifyRecoveryIntegrityLocked(manifest); err != nil {
		return err
	}
	if manifest.Active {
		if refresh {
			return errors.New("cannot replace backup while Potato Mode restoration is pending")
		}
		return nil
	}
	if !refresh {
		if _, err := os.Stat(filepath.Join(m.backupDir, "GameUserSettings.ini.original")); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat Squad baseline backup: %w", err)
		}
	}
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("read Squad settings for backup: %w", err)
	}
	if !force && looksLikePotatoConfig(data) {
		return errors.New("current Squad settings appear to be Potato Mode; refusing to replace the original backup")
	}
	if err := os.MkdirAll(m.backupDir, 0o700); err != nil {
		return fmt.Errorf("create Squad backup directory: %w", err)
	}
	_ = m.ensureInitialBaselineCapturedLocked()
	if err := m.archiveLegacyBackupLocked(manifest); err != nil {
		return err
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(m.backupDir, "GameUserSettings.ini.original"), data, 0o600); err != nil {
		return fmt.Errorf("write Squad baseline backup: %w", err)
	}
	manifest = BackupManifest{
		Version:      backupManifestVersion,
		OriginalPath: m.configPath,
		Sha256:       checksum(data),
		Timestamp:    time.Now().UTC(),
		SizeBytes:    int64(len(data)),
	}
	return m.writeManifestLocked(manifest)
}

func looksLikePotatoConfig(data []byte) bool {
	settings, _ := iniSettings(data)
	if marker, exists := settings["citadelsentry\x00potatomode"]; exists {
		if enabled, err := strconv.ParseBool(marker.value); err == nil && enabled {
			return true
		}
	}
	// Earlier Sentry versions wrote this combination without a marker.
	hasLegacyQuality := hasIniNumber(settings, "systemsettings\x00t.maxfps", 1) || hasIniNumber(settings, "scalabilitygroups\x00sg.resolutionquality", 25)
	if !hasLegacyQuality {
		return false
	}
	for _, section := range []string{squadUserSettingsSection, "/script/engine.gameusersettings", "/script/squad.squadgameusersettings"} {
		if !hasIniNumber(settings, section+"\x00frameratelimit", 1) {
			continue
		}
		vsync, exists := settings[section+"\x00busevsync"]
		if !exists {
			continue
		}
		if enabled, err := strconv.ParseBool(vsync.value); err == nil && !enabled {
			return true
		}
	}
	return false
}

func hasIniNumber(settings map[string]iniSetting, id string, expected float64) bool {
	setting, exists := settings[id]
	if !exists {
		return false
	}
	value, err := strconv.ParseFloat(setting.value, 64)
	return err == nil && value == expected
}

func (m *ConfigManager) patchTargetsLocked() ([]string, error) {
	if m.configPath == "" {
		return nil, errors.New("Squad GameUserSettings.ini path not detected")
	}
	var paths []string
	seen := make(map[string]bool)
	addPath := func(path string, isRequired bool) error {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) && !isRequired {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect Squad settings %s: %w", path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("Squad settings path is a directory: %s", path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Squad settings must be a regular file; linked or special files are not replaced: %s", path)
		}
		if err := fileutil.VerifySingleFileLink(path, info); err != nil {
			return fmt.Errorf("inspect Squad settings links: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve Squad settings path %s: %w", path, err)
		}
		resolved = filepath.Clean(resolved)
		if seen[resolved] {
			return nil
		}
		seen[resolved] = true
		paths = append(paths, path)
		return nil
	}
	if err := addPath(m.configPath, true); err != nil {
		return nil, err
	}
	for _, path := range m.secondaryPaths {
		if err := addPath(path, false); err != nil {
			return nil, err
		}
	}
	settingsPaths := append([]string(nil), paths...)
	for _, path := range settingsPaths {
		if err := addPath(filepath.Join(filepath.Dir(path), "Engine.ini"), false); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func (m *ConfigManager) applyPotatoLocked(fpsLimit int, lowResolution bool, gpu ...platform.GPUInfo) error {
	if fpsLimit <= 0 || fpsLimit > 60 {
		return fmt.Errorf("invalid Potato Mode FPS limit %d", fpsLimit)
	}
	manifest, err := m.readManifestLocked()
	if err != nil {
		return err
	}
	if err := m.verifyRecoveryIntegrityLocked(manifest); err != nil {
		return err
	}
	if manifest.Active {
		return errors.New("Potato Mode restoration is already pending")
	}
	targets, err := m.patchTargetsLocked()
	if err != nil {
		return err
	}
	patches := make([]configPatch, 0, len(targets))
	for index, path := range targets {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("inspect original Squad config %s: %w", path, errors.Join(err, errors.New("original must be a regular file")))
		}
		if info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
			return fmt.Errorf("unsupported special Squad config permissions: %s", path)
		}
		original, err := os.ReadFile(path)
		existed := err == nil
		if err != nil {
			return fmt.Errorf("read Squad config %s: %w", path, err)
		}
		if path == m.configPath && looksLikePotatoConfig(original) {
			return errors.New("Squad settings already look like Potato Mode; restore them before starting a new session")
		}
		var patched []byte
		if filepath.Base(path) == "Engine.ini" {
			settings, duplicates := iniSettings(original)
			if duplicates["systemsettings\x00t.maxfps"] {
				return fmt.Errorf("repeated t.MaxFPS in Squad engine settings %s", path)
			}
			if setting, exists := settings["systemsettings\x00t.maxfps"]; exists {
				if err := validateNumericSetting(setting); err != nil {
					return fmt.Errorf("unsupported Squad engine settings %s: %w", path, err)
				}
			}
			patched = patchEngineIni(original, fpsLimit)
		} else {
			if err := validatePotatoSettings(original, fpsLimit, lowResolution, gpu...); err != nil {
				if path != m.configPath {
					logger.Warn("Skipping secondary Squad config that cannot be patched", "path", path, "error", err)
					continue
				}
				return fmt.Errorf("unsupported Squad settings %s: %w", path, err)
			}
			patched = patchPotatoIni(original, fpsLimit, lowResolution, gpu...)
		}
		if bytes.Equal(original, patched) {
			continue
		}
		entry := BackupEntry{Path: path, Existed: existed, PatchedSha256: checksum(patched)}
		entry.PatchedName = fmt.Sprintf("session-%02d-%s.patched", index, filepath.Base(path))
		if existed {
			mode := uint32(info.Mode().Perm())
			entry.OriginalMode = &mode
			entry.BackupName = fmt.Sprintf("session-%02d-%s.original", index, filepath.Base(path))
			if path == m.configPath {
				entry.BackupName = "GameUserSettings.ini.original"
			}
			entry.Sha256 = checksum(original)
			entry.SizeBytes = int64(len(original))
		}
		patches = append(patches, configPatch{entry: entry, original: original, patched: patched})
	}
	if len(patches) == 0 {
		return nil
	}
	if err := os.MkdirAll(m.backupDir, 0o700); err != nil {
		return fmt.Errorf("create Squad backup directory: %w", err)
	}
	_ = m.ensureInitialBaselineCapturedLocked()
	if err := m.archiveLegacyBackupLocked(manifest); err != nil {
		return err
	}
	manifest = BackupManifest{
		Version:      backupManifestVersion,
		OriginalPath: m.configPath,
		Timestamp:    time.Now().UTC(),
		Active:       true,
		Files:        make([]BackupEntry, 0, len(patches)),
	}
	for _, patch := range patches {
		manifest.Files = append(manifest.Files, patch.entry)
		if err := fileutil.WriteFileAtomic(filepath.Join(m.backupDir, patch.entry.PatchedName), patch.patched, 0o600); err != nil {
			return fmt.Errorf("save expected Potato Mode patch for %s: %w", patch.entry.Path, err)
		}
		if !patch.entry.Existed {
			continue
		}
		if err := fileutil.WriteFileAtomic(filepath.Join(m.backupDir, patch.entry.BackupName), patch.original, 0o600); err != nil {
			return fmt.Errorf("back up %s: %w", patch.entry.Path, err)
		}
		if patch.entry.Path == m.configPath {
			manifest.Sha256 = patch.entry.Sha256
			manifest.SizeBytes = patch.entry.SizeBytes
		}
	}
	if err := m.writeManifestLocked(manifest); err != nil {
		return err
	}
	for _, patch := range patches {
		current, readErr := os.ReadFile(patch.entry.Path)
		if patch.entry.Existed && (readErr != nil || checksum(current) != patch.entry.Sha256) {
			rollbackErr := m.restoreOriginalLocked()
			return errors.Join(fmt.Errorf("Squad config changed before Potato Mode patch: %s", patch.entry.Path), rollbackErr)
		}
		if !patch.entry.Existed && !errors.Is(readErr, os.ErrNotExist) {
			rollbackErr := m.restoreOriginalLocked()
			return errors.Join(fmt.Errorf("Squad config appeared before Potato Mode patch: %s", patch.entry.Path), rollbackErr)
		}
		mode := os.FileMode(0o600)
		if patch.entry.OriginalMode != nil {
			mode = writableConfigMode(os.FileMode(*patch.entry.OriginalMode))
		}
		if err := writeConfigPatch(patch.entry.Path, patch.patched, patch.entry.Existed, mode); err != nil {
			rollbackErr := m.restoreOriginalLocked()
			return errors.Join(fmt.Errorf("apply Potato Mode: %w", err), rollbackErr)
		}
	}
	return nil
}

func patchEngineIni(original []byte, fpsLimit int) []byte {
	if len(original) == 0 {
		return original
	}
	text := string(original)
	engineSettings := []struct{ key, val string }{
		{"r.Streaming.PoolSize", "150"},
		{"r.Streaming.LimitPoolSizeToVRAM", "1"},
		{"r.Streaming.UseFixedPoolSize", "1"},
		{"r.Streaming.MaxTempMemoryAllowed", "20"},
		{"r.Shaders.Optimize", "1"},
		{"r.CreateShadersOnLoad", "0"},
	}
	for _, s := range engineSettings {
		text = setIniSectionKey(text, "SystemSettings", s.key, s.val)
	}
	return []byte(text)
}

func writeConfigPatch(path string, data []byte, existed bool, mode os.FileMode) error {
	if existed {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect config before replacement %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace a linked or special Squad config: %s", path)
		}
		if err := fileutil.VerifySingleFileLink(path, info); err != nil {
			return fmt.Errorf("inspect Squad config links before replacement: %w", err)
		}
	} else if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to replace newly created Squad config %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect newly created Squad config %s: %w", path, err)
	}
	writeFile := fileutil.WriteFileAtomic
	if !existed {
		writeFile = fileutil.CreateFileAtomic
	}
	if err := writeFile(path, data, mode); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verify %s: %w", path, err)
	}
	if checksum(actual) != checksum(data) {
		return fmt.Errorf("verify %s: checksum mismatch", path)
	}
	return nil
}

func (m *ConfigManager) restoreOriginalLocked() error {
	manifest, err := m.readManifestLocked()
	if err != nil {
		return err
	}
	if err := m.verifyRecoveryIntegrityLocked(manifest); err != nil {
		return err
	}
	if !manifest.Active {
		return nil
	}
	if manifest.Version != backupManifestVersion || manifest.OriginalPath != m.configPath || len(manifest.Files) == 0 {
		return errors.New("Squad backup manifest does not match the active configuration")
	}
	type restoreItem struct {
		entry        BackupEntry
		backup       []byte
		current      []byte
		action       string
		observedHash string
		mode         os.FileMode
		observedMode os.FileMode
	}
	items := make([]restoreItem, 0, len(manifest.Files))
	for _, entry := range manifest.Files {
		if err := m.validateEntryPathLocked(entry.Path); err != nil {
			return err
		}
		var backup []byte
		var originalMode os.FileMode
		if entry.Existed {
			if entry.BackupName == "" || filepath.Base(entry.BackupName) != entry.BackupName {
				return fmt.Errorf("invalid backup name for %s", entry.Path)
			}
			backup, err = os.ReadFile(filepath.Join(m.backupDir, entry.BackupName))
			if err != nil {
				return fmt.Errorf("read Squad backup for %s: %w", entry.Path, err)
			}
			if int64(len(backup)) != entry.SizeBytes || checksum(backup) != entry.Sha256 {
				return fmt.Errorf("Squad backup size or checksum mismatch for %s", entry.Path)
			}
			backupInfo, err := os.Stat(filepath.Join(m.backupDir, entry.BackupName))
			if err != nil {
				return fmt.Errorf("inspect Squad backup permissions for %s: %w", entry.Path, err)
			}
			originalMode = backupInfo.Mode().Perm()
			if entry.OriginalMode != nil {
				originalMode = os.FileMode(*entry.OriginalMode)
			}
		}
		var patched []byte
		if entry.PatchedName != "" {
			if filepath.Base(entry.PatchedName) != entry.PatchedName {
				return fmt.Errorf("invalid patched file name for %s", entry.Path)
			}
			patched, err = os.ReadFile(filepath.Join(m.backupDir, entry.PatchedName))
			if err != nil {
				return fmt.Errorf("read Potato Mode patch for %s: %w", entry.Path, err)
			}
			if checksum(patched) != entry.PatchedSha256 {
				return fmt.Errorf("Potato Mode patch checksum mismatch for %s", entry.Path)
			}
		}
		currentInfo, inspectErr := os.Lstat(entry.Path)
		if inspectErr == nil && !currentInfo.Mode().IsRegular() {
			return fmt.Errorf("refusing recovery of a linked or special Squad config: %s", entry.Path)
		}
		if inspectErr == nil {
			if err := fileutil.VerifySingleFileLink(entry.Path, currentInfo); err != nil {
				return fmt.Errorf("inspect Squad config links before recovery: %w", err)
			}
		}
		if inspectErr != nil && !errors.Is(inspectErr, os.ErrNotExist) {
			return fmt.Errorf("inspect current Squad config %s: %w", entry.Path, inspectErr)
		}
		if errors.Is(inspectErr, os.ErrNotExist) {
			item := restoreItem{entry: entry, backup: backup, mode: originalMode}
			if entry.Existed {
				item.action = "create"
			}
			items = append(items, item)
			continue
		}
		current, readErr := os.ReadFile(entry.Path)
		if errors.Is(readErr, os.ErrNotExist) {
			item := restoreItem{entry: entry, backup: backup, mode: originalMode}
			if entry.Existed {
				item.action = "create"
			}
			items = append(items, item)
			continue
		}
		if readErr != nil {
			return fmt.Errorf("read current Squad config %s: %w", entry.Path, readErr)
		}
		currentHash := checksum(current)
		item := restoreItem{entry: entry, backup: backup, current: current, observedHash: currentHash, observedMode: currentInfo.Mode().Perm()}
		item.mode = restoredConfigMode(entry, item.observedMode)
		if entry.Existed && currentHash == entry.Sha256 {
			if item.mode != item.observedMode {
				item.action = "permissions"
			}
			items = append(items, item)
			continue
		}
		if currentHash == entry.PatchedSha256 {
			item.action = "restore"
			items = append(items, item)
			continue
		}
		if entry.PatchedName == "" {
			return fmt.Errorf("Squad changed %s during Potato Mode; refusing to overwrite player changes", entry.Path)
		}
		merged, mergeErr := mergePotatoIni(backup, patched, current)
		if mergeErr != nil {
			return fmt.Errorf("merge Squad changes in %s: %w", entry.Path, mergeErr)
		}
		item.current = merged
		item.action = "merge"
		items = append(items, item)
	}
	for _, item := range items {
		if item.action == "" {
			continue
		}
		if item.action == "create" {
			if err := writeConfigPatch(item.entry.Path, item.backup, false, item.mode); err != nil {
				return fmt.Errorf("restore missing %s: %w", item.entry.Path, err)
			}
			continue
		}
		observed, err := os.ReadFile(item.entry.Path)
		if err != nil || checksum(observed) != item.observedHash {
			return fmt.Errorf("Squad config changed during restoration: %s", item.entry.Path)
		}
		info, err := os.Lstat(item.entry.Path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != item.observedMode {
			return fmt.Errorf("Squad config attributes changed during restoration: %s", item.entry.Path)
		}
		if item.action == "permissions" {
			if err := os.Chmod(item.entry.Path, item.mode); err != nil {
				return fmt.Errorf("restore original permissions for %s: %w", item.entry.Path, err)
			}
			continue
		}
		if item.action == "merge" {
			if err := writeConfigPatch(item.entry.Path, item.current, true, item.mode); err != nil {
				return fmt.Errorf("merge changes in %s: %w", item.entry.Path, err)
			}
			continue
		}
		if item.entry.Existed {
			if err := writeConfigPatch(item.entry.Path, item.backup, true, item.mode); err != nil {
				return fmt.Errorf("restore %s: %w", item.entry.Path, err)
			}
			continue
		}
		if err := os.Remove(item.entry.Path); err != nil {
			return fmt.Errorf("remove Potato Mode-created %s: %w", item.entry.Path, err)
		}
	}
	manifest.Active = false
	return m.writeManifestLocked(manifest)
}

func (m *ConfigManager) validateEntryPathLocked(path string) error {
	allowed := map[string]bool{}
	settingsPaths := append([]string{m.configPath}, m.secondaryPaths...)
	settingsPaths = append(settingsPaths, getSquadConfigCandidates()...)
	for _, settingsPath := range settingsPaths {
		allowed[filepath.Clean(settingsPath)] = true
		allowed[filepath.Clean(filepath.Join(filepath.Dir(settingsPath), "Engine.ini"))] = true
	}
	if !allowed[filepath.Clean(path)] {
		return fmt.Errorf("backup references unexpected Squad config path %s", path)
	}
	return nil
}

func (m *ConfigManager) updateActiveFpsLocked(fpsLimit int) error {
	if fpsLimit <= 0 || fpsLimit > 144 {
		return fmt.Errorf("invalid FPS limit: %d", fpsLimit)
	}
	manifest, err := m.readManifestLocked()
	if err != nil {
		return err
	}
	if !manifest.Active {
		return errors.New("no active Potato Mode session to update")
	}
	for i, entry := range manifest.Files {
		current, err := os.ReadFile(entry.Path)
		if err != nil {
			return fmt.Errorf("read active config %s: %w", entry.Path, err)
		}
		var patched []byte
		if filepath.Base(entry.Path) == "Engine.ini" {
			patched = patchEngineIni(current, fpsLimit)
		} else {
			text := string(current)
			newFpsVal := fmt.Sprintf("%.6f", float64(fpsLimit))
			settings, _ := iniSettings(current)
			for _, setting := range settings {
				if strings.ToLower(setting.key) == "frameratelimit" {
					text = setIniSectionKey(text, setting.section, setting.key, newFpsVal)
				}
			}
			patched = []byte(text)
		}
		if err := fileutil.WriteFileAtomic(entry.Path, patched, 0o600); err != nil {
			return fmt.Errorf("write updated active config %s: %w", entry.Path, err)
		}
		manifest.Files[i].PatchedSha256 = checksum(patched)
		if entry.PatchedName != "" {
			_ = fileutil.WriteFileAtomic(filepath.Join(m.backupDir, entry.PatchedName), patched, 0o600)
		}
	}
	return m.writeManifestLocked(manifest)
}
