package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"citadel-seeder/internal/fileutil"
)

// BaselineMetadata stores metadata for the permanent, immutable first-launch baseline.
type BaselineMetadata struct {
	Version      int           `json:"version"`
	CapturedAt   time.Time     `json:"capturedAt"`
	OriginalPath string        `json:"originalPath"`
	Files        []BackupEntry `json:"files"`
	Sha256       string        `json:"sha256"`
	SizeBytes    int64         `json:"sizeBytes"`
}

// RestorePoint represents a user-created snapshot of their Squad configuration.
type RestorePoint struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	Files       []BackupEntry `json:"files"`
	Sha256      string        `json:"sha256"`
	SizeBytes   int64         `json:"sizeBytes"`
}

func (m *ConfigManager) baselineDir() string {
	return filepath.Join(m.backupDir, "baseline")
}

func (m *ConfigManager) baselineMetaPath() string {
	return filepath.Join(m.baselineDir(), "baseline.json")
}

func (m *ConfigManager) restorePointsDir() string {
	return filepath.Join(m.backupDir, "restore_points")
}

// HasInitialBaseline checks if an immutable first-launch baseline exists.
func (m *ConfigManager) HasInitialBaseline() bool {
	info, err := os.Lstat(m.baselineMetaPath())
	return err == nil && info.Mode().IsRegular()
}

// EnsureInitialBaselineCaptured captures the baseline upon first launch.
// It can NEVER overwrite an existing baseline.
func (m *ConfigManager) EnsureInitialBaselineCaptured() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureInitialBaselineCapturedLocked()
}

func (m *ConfigManager) ensureInitialBaselineCapturedLocked() error {
	if m.HasInitialBaseline() {
		return nil // Immutable: never overwrite
	}
	if m.configPath == "" {
		return nil
	}
	info, err := os.Lstat(m.configPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(m.configPath)
	if err != nil || looksLikePotatoConfig(data) {
		return nil // Do not capture a potato config as the original baseline
	}

	bDir := m.baselineDir()
	if err := os.MkdirAll(bDir, 0o700); err != nil {
		return fmt.Errorf("create baseline directory: %w", err)
	}

	entries := []BackupEntry{}

	// Capture primary GameUserSettings.ini
	baseName := filepath.Base(m.configPath)
	destPath := filepath.Join(bDir, baseName)
	if err := fileutil.WriteFileAtomic(destPath, data, 0o600); err != nil {
		return fmt.Errorf("write baseline %s: %w", baseName, err)
	}
	entries = append(entries, BackupEntry{
		Path:       m.configPath,
		BackupName: baseName,
		Sha256:     checksum(data),
		SizeBytes:  int64(len(data)),
		Existed:    true,
	})

	// Also capture Engine.ini if present in the same directory
	enginePath := filepath.Join(filepath.Dir(m.configPath), "Engine.ini")
	if eInfo, err := os.Lstat(enginePath); err == nil && eInfo.Mode().IsRegular() {
		if eData, err := os.ReadFile(enginePath); err == nil {
			eName := "Engine.ini"
			eDest := filepath.Join(bDir, eName)
			if err := fileutil.WriteFileAtomic(eDest, eData, 0o600); err == nil {
				entries = append(entries, BackupEntry{
					Path:       enginePath,
					BackupName: eName,
					Sha256:     checksum(eData),
					SizeBytes:  int64(len(eData)),
					Existed:    true,
				})
			}
		}
	}

	meta := BaselineMetadata{
		Version:      backupManifestVersion,
		CapturedAt:   time.Now().UTC(),
		OriginalPath: m.configPath,
		Files:        entries,
		Sha256:       checksum(data),
		SizeBytes:    int64(len(data)),
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline metadata: %w", err)
	}
	if err := fileutil.WriteFileAtomic(m.baselineMetaPath(), metaBytes, 0o600); err != nil {
		return fmt.Errorf("write baseline metadata: %w", err)
	}
	return nil
}

// RestoreInitialBaseline restores the Squad configuration back to its immutable first-launch baseline.
func (m *ConfigManager) RestoreInitialBaseline() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.HasInitialBaseline() {
		return errors.New("initial baseline not found")
	}

	metaBytes, err := os.ReadFile(m.baselineMetaPath())
	if err != nil {
		return fmt.Errorf("read baseline metadata: %w", err)
	}
	var meta BaselineMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return fmt.Errorf("decode baseline metadata: %w", err)
	}

	bDir := m.baselineDir()
	// Verify integrity and path validity first
	for _, entry := range meta.Files {
		if entry.BackupName == "" || filepath.Base(entry.BackupName) != entry.BackupName {
			return fmt.Errorf("invalid baseline backup name: %s", entry.BackupName)
		}
		if err := m.validateEntryPathLocked(entry.Path); err != nil {
			return err
		}
		backupPath := filepath.Join(bDir, entry.BackupName)
		content, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read baseline file %s: %w", entry.BackupName, err)
		}
		if checksum(content) != entry.Sha256 {
			return fmt.Errorf("baseline file %s checksum mismatch", entry.BackupName)
		}
	}

	// Restore each file
	for _, entry := range meta.Files {
		content, err := os.ReadFile(filepath.Join(bDir, entry.BackupName))
		if err != nil {
			return fmt.Errorf("read baseline file for restore: %w", err)
		}
		if err := fileutil.WriteFileAtomic(entry.Path, content, 0o600); err != nil {
			return fmt.Errorf("restore baseline to %s: %w", entry.Path, err)
		}
	}

	// Deactivate any active potato session in the manifest
	if manifest, err := m.readManifestLocked(); err == nil && manifest.Active {
		manifest.Active = false
		_ = m.writeManifestLocked(manifest)
	}

	m.refreshDetectedPathsLocked()
	return nil
}

// CreateRestorePoint captures a named snapshot of current settings.
func (m *ConfigManager) CreateRestorePoint(name, description string) (RestorePoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.configPath == "" {
		return RestorePoint{}, errors.New("squad configuration not found")
	}
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return RestorePoint{}, fmt.Errorf("read squad config: %w", err)
	}
	if looksLikePotatoConfig(data) {
		return RestorePoint{}, errors.New("cannot create restore point while Potato Mode is active; restore normal settings first")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Snapshot %s", time.Now().Format("2006-01-02 15:04"))
	}

	id := fmt.Sprintf("rp-%d", time.Now().UnixNano())
	rpDir := filepath.Join(m.restorePointsDir(), id)
	if err := os.MkdirAll(rpDir, 0o700); err != nil {
		return RestorePoint{}, fmt.Errorf("create restore point directory: %w", err)
	}

	entries := []BackupEntry{}
	baseName := filepath.Base(m.configPath)
	destPath := filepath.Join(rpDir, baseName)
	if err := fileutil.WriteFileAtomic(destPath, data, 0o600); err != nil {
		_ = os.RemoveAll(rpDir)
		return RestorePoint{}, fmt.Errorf("save restore point config: %w", err)
	}
	entries = append(entries, BackupEntry{
		Path:       m.configPath,
		BackupName: baseName,
		Sha256:     checksum(data),
		SizeBytes:  int64(len(data)),
		Existed:    true,
	})

	// Also include Engine.ini if present
	enginePath := filepath.Join(filepath.Dir(m.configPath), "Engine.ini")
	if eInfo, err := os.Lstat(enginePath); err == nil && eInfo.Mode().IsRegular() {
		if eData, err := os.ReadFile(enginePath); err == nil {
			eName := "Engine.ini"
			eDest := filepath.Join(rpDir, eName)
			if err := fileutil.WriteFileAtomic(eDest, eData, 0o600); err == nil {
				entries = append(entries, BackupEntry{
					Path:       enginePath,
					BackupName: eName,
					Sha256:     checksum(eData),
					SizeBytes:  int64(len(eData)),
					Existed:    true,
				})
			}
		}
	}

	rp := RestorePoint{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		Files:       entries,
		Sha256:      checksum(data),
		SizeBytes:   int64(len(data)),
	}

	metaBytes, err := json.MarshalIndent(rp, "", "  ")
	if err != nil {
		_ = os.RemoveAll(rpDir)
		return RestorePoint{}, fmt.Errorf("encode restore point metadata: %w", err)
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(rpDir, "restore_point.json"), metaBytes, 0o600); err != nil {
		_ = os.RemoveAll(rpDir)
		return RestorePoint{}, fmt.Errorf("save restore point metadata: %w", err)
	}

	return rp, nil
}

// ListRestorePoints returns all saved snapshots sorted by creation date (newest first).
func (m *ConfigManager) ListRestorePoints() ([]RestorePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := m.restorePointsDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []RestorePoint{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read restore points directory: %w", err)
	}

	points := make([]RestorePoint, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, entry.Name(), "restore_point.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var rp RestorePoint
		if err := json.Unmarshal(data, &rp); err == nil {
			points = append(points, rp)
		}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].CreatedAt.After(points[j].CreatedAt)
	})

	return points, nil
}

// RestoreRestorePoint restores settings from a specific named snapshot.
func (m *ConfigManager) RestoreRestorePoint(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || !strings.HasPrefix(id, "rp-") {
		return errors.New("invalid restore point identifier")
	}

	rpDir := filepath.Join(m.restorePointsDir(), id)
	metaPath := filepath.Join(rpDir, "restore_point.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("read restore point %s: %w", id, err)
	}
	var rp RestorePoint
	if err := json.Unmarshal(data, &rp); err != nil {
		return fmt.Errorf("decode restore point %s: %w", id, err)
	}

	// Verify files and validate paths
	for _, entry := range rp.Files {
		if entry.BackupName == "" || filepath.Base(entry.BackupName) != entry.BackupName {
			return fmt.Errorf("invalid snapshot file name: %s", entry.BackupName)
		}
		if err := m.validateEntryPathLocked(entry.Path); err != nil {
			return err
		}
		src := filepath.Join(rpDir, entry.BackupName)
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read snapshot file %s: %w", entry.BackupName, err)
		}
		if checksum(content) != entry.Sha256 {
			return fmt.Errorf("snapshot file %s checksum mismatch", entry.BackupName)
		}
	}

	// Restore files
	for _, entry := range rp.Files {
		content, err := os.ReadFile(filepath.Join(rpDir, entry.BackupName))
		if err != nil {
			return fmt.Errorf("read snapshot file for restore: %w", err)
		}
		if err := fileutil.WriteFileAtomic(entry.Path, content, 0o600); err != nil {
			return fmt.Errorf("restore snapshot file to %s: %w", entry.Path, err)
		}
	}

	// Deactivate any active potato session
	if manifest, err := m.readManifestLocked(); err == nil && manifest.Active {
		manifest.Active = false
		_ = m.writeManifestLocked(manifest)
	}

	m.refreshDetectedPathsLocked()
	return nil
}

// DeleteRestorePoint deletes a named snapshot.
func (m *ConfigManager) DeleteRestorePoint(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || !strings.HasPrefix(id, "rp-") {
		return errors.New("invalid restore point identifier")
	}

	rpDir := filepath.Join(m.restorePointsDir(), id)
	return os.RemoveAll(rpDir)
}
