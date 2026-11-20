package game

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"citadel-seeder/internal/fileutil"
)

const maxSquadManifestBytes = 1 << 20

func decodeSquadManifest(data []byte) (BackupManifest, error) {
	var fields map[string]json.RawMessage
	if err := fileutil.DecodeJSONStrict(data, &fields); err != nil {
		return BackupManifest{}, err
	}
	if fields == nil {
		return BackupManifest{}, fmt.Errorf("Squad backup manifest must be a JSON object")
	}
	for key, value := range fields {
		switch key {
		case "version", "originalPath", "sha256", "timestamp", "sizeBytes", "active", "canKeepAfterSession", "files":
		default:
			return BackupManifest{}, fmt.Errorf("unknown or noncanonical Squad backup manifest field %s", key)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return BackupManifest{}, fmt.Errorf("Squad backup manifest field %s is null", key)
		}
	}
	var manifest BackupManifest
	if err := fileutil.DecodeJSONStrict(data, &manifest); err != nil {
		return BackupManifest{}, err
	}
	if manifest.Version != 0 && manifest.Version != backupManifestVersion {
		return BackupManifest{}, fmt.Errorf("unsupported Squad backup manifest version %d", manifest.Version)
	}
	if manifest.OriginalPath == "" || !filepath.IsAbs(manifest.OriginalPath) {
		return BackupManifest{}, fmt.Errorf("Squad backup originalPath must be absolute")
	}
	if manifest.SizeBytes < 0 {
		return BackupManifest{}, fmt.Errorf("Squad backup size cannot be negative")
	}
	if manifest.Version == 0 {
		if len(manifest.Files) != 0 || manifest.CanKeepAfterSession {
			return BackupManifest{}, fmt.Errorf("transaction records require an explicit supported manifest version")
		}
		if manifest.Sha256 != "" && !isSquadChecksum(manifest.Sha256) {
			return BackupManifest{}, fmt.Errorf("invalid legacy Squad backup checksum")
		}
		return manifest, nil
	}
	if err := requireManifestFields(fields, "manifest", []string{"version", "originalPath", "sha256", "timestamp", "sizeBytes", "active"}); err != nil {
		return BackupManifest{}, err
	}
	if manifest.Active && len(manifest.Files) == 0 {
		return BackupManifest{}, fmt.Errorf("active Squad backup has no file records")
	}
	if len(manifest.Files) == 0 && !isSquadChecksum(manifest.Sha256) {
		return BackupManifest{}, fmt.Errorf("Squad baseline backup checksum is invalid")
	}
	if len(manifest.Files) > 0 && manifest.Sha256 != "" && !isSquadChecksum(manifest.Sha256) {
		return BackupManifest{}, fmt.Errorf("Squad session backup checksum is invalid")
	}
	var entries []map[string]json.RawMessage
	if value, exists := fields["files"]; exists {
		if err := json.Unmarshal(value, &entries); err != nil {
			return BackupManifest{}, err
		}
	}
	if len(entries) != len(manifest.Files) {
		return BackupManifest{}, fmt.Errorf("Squad backup file records do not match their raw metadata")
	}
	seenPaths := make(map[string]bool)
	seenNames := make(map[string]bool)
	for index, entry := range manifest.Files {
		if err := requireManifestFields(entries[index], fmt.Sprintf("file %d", index), []string{"path", "sizeBytes", "patchedSha256", "existed"}); err != nil {
			return BackupManifest{}, err
		}
		for key, value := range entries[index] {
			switch key {
			case "path", "backupName", "patchedName", "sha256", "sizeBytes", "patchedSha256", "existed", "originalMode", "previousPatches":
			default:
				return BackupManifest{}, fmt.Errorf("unknown or noncanonical Squad backup file %d field %s", index, key)
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return BackupManifest{}, fmt.Errorf("Squad backup file %d field %s is null", index, key)
			}
		}
		path := filepath.Clean(entry.Path)
		if !filepath.IsAbs(entry.Path) || seenPaths[path] || !isSquadChecksum(entry.PatchedSha256) || entry.SizeBytes < 0 {
			return BackupManifest{}, fmt.Errorf("invalid or repeated Squad backup file record %d", index)
		}
		seenPaths[path] = true
		if entry.Existed {
			if err := requireManifestFields(entries[index], fmt.Sprintf("file %d", index), []string{"backupName", "sha256"}); err != nil {
				return BackupManifest{}, err
			}
			if !isSquadChecksum(entry.Sha256) {
				return BackupManifest{}, fmt.Errorf("invalid original checksum for Squad backup file %d", index)
			}
			if entry.OriginalMode != nil && *entry.OriginalMode > 0o777 {
				return BackupManifest{}, fmt.Errorf("invalid original permissions for Squad backup file %d", index)
			}
		} else if entry.BackupName != "" || entry.Sha256 != "" || entry.SizeBytes != 0 || entry.OriginalMode != nil {
			return BackupManifest{}, fmt.Errorf("created Squad file %d has conflicting original metadata", index)
		}
		for _, name := range []string{entry.BackupName, entry.PatchedName} {
			if name == "" {
				continue
			}
			if filepath.Base(name) != name || name == "." || name == ".." || seenNames[name] {
				return BackupManifest{}, fmt.Errorf("invalid or repeated Squad backup data name %q", name)
			}
			seenNames[name] = true
		}
		for prevIndex, prev := range entry.PreviousPatches {
			if prev.Name == "" || filepath.Base(prev.Name) != prev.Name || prev.Name == "." || prev.Name == ".." || seenNames[prev.Name] {
				return BackupManifest{}, fmt.Errorf("invalid or repeated Squad previous patch name %q in file %d", prev.Name, index)
			}
			if !isSquadChecksum(prev.Sha256) {
				return BackupManifest{}, fmt.Errorf("invalid checksum for previous patch %q in file %d (index %d)", prev.Name, index, prevIndex)
			}
			seenNames[prev.Name] = true
		}
		if entry.Path == manifest.OriginalPath && (entry.Sha256 != manifest.Sha256 || entry.SizeBytes != manifest.SizeBytes || !entry.Existed) {
			return BackupManifest{}, fmt.Errorf("primary Squad backup metadata does not match its file record")
		}
	}
	return manifest, nil
}

func requireManifestFields(fields map[string]json.RawMessage, name string, required []string) error {
	for _, key := range required {
		value, exists := fields[key]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("Squad backup %s field %s is missing or null", name, key)
		}
	}
	return nil
}

func isSquadChecksum(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 32
}
