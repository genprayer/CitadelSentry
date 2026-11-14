package game

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// verifyRecoveryIntegrityLocked checks every recorded original and expected patch.
// Inactive legacy records without checksums can only be preserved in an archive.
func (m *ConfigManager) verifyRecoveryIntegrityLocked(manifest BackupManifest) error {
	if manifest.Version != backupManifestVersion && !manifest.Active && manifest.Sha256 == "" {
		return nil
	}
	return m.verifyBackupIntegrityLocked(manifest)
}

func (m *ConfigManager) verifyBackupIntegrityLocked(manifest BackupManifest) error {
	if manifest.OriginalPath == "" {
		return nil
	}
	if len(manifest.Files) == 0 {
		return verifySquadBackupFile(filepath.Join(m.backupDir, "GameUserSettings.ini.original"), manifest.Sha256, &manifest.SizeBytes)
	}
	for _, entry := range manifest.Files {
		if entry.Existed {
			if err := verifySquadBackupFile(filepath.Join(m.backupDir, entry.BackupName), entry.Sha256, &entry.SizeBytes); err != nil {
				return err
			}
		}
		if entry.PatchedName != "" {
			if err := verifySquadBackupFile(filepath.Join(m.backupDir, entry.PatchedName), entry.PatchedSha256, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifySquadBackupFile streams the checksum without loading another full copy.
func verifySquadBackupFile(path, expectedHash string, expectedSize *int64) error {
	if !isSquadChecksum(expectedHash) {
		return fmt.Errorf("Squad backup has no verified checksum: %s", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect Squad backup record %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Squad backup record is not a regular file: %s", path)
	}
	if expectedSize != nil && info.Size() != *expectedSize {
		return fmt.Errorf("Squad backup size or checksum mismatch: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Squad backup record %s: %w", path, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return fmt.Errorf("Squad backup record changed before verification: %s", path)
	}
	digest := sha256.New()
	count, err := io.Copy(digest, file)
	if err != nil {
		return fmt.Errorf("read Squad backup record %s: %w", path, err)
	}
	if count != info.Size() || hex.EncodeToString(digest.Sum(nil)) != expectedHash {
		return fmt.Errorf("Squad backup checksum or size mismatch: %s", path)
	}
	return nil
}
