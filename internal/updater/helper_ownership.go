package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"citadel-seeder/internal/fileutil"
)

const (
	helperOwnershipFile     = "helper-ownership.json"
	helperOwnershipVersion  = 1
	maxHelperOwnershipBytes = 8 << 10
	helperPhasePrepared     = "prepared"
	helperPhaseCleaning     = "cleaning"
)

type helperOwnership struct {
	Version    int    `json:"version"`
	Target     string `json:"target"`
	HelperName string `json:"helperName"`
	HelperHash string `json:"helperHash"`
	UpdateHash string `json:"updateHash"`
	Phase      string `json:"phase"`
}

func writeHelperOwnership(directory, target, helper, updateHash string) error {
	if !isHelperDirectory(directory, target) || filepath.Dir(helper) != directory || !hashPattern.MatchString(updateHash) {
		return errors.New("helper ownership paths or update hash are invalid")
	}
	helperHash, err := fileHash(helper)
	if err != nil {
		return err
	}
	record := helperOwnership{Version: helperOwnershipVersion, Target: target, HelperName: filepath.Base(helper), HelperHash: helperHash, UpdateHash: updateHash, Phase: helperPhasePrepared}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return fileutil.CreateFileAtomic(filepath.Join(directory, helperOwnershipFile), data, 0o600)
}

func readHelperOwnership(directory, target string) (helperOwnership, error) {
	path := filepath.Join(directory, helperOwnershipFile)
	info, err := os.Lstat(path)
	if err != nil {
		return helperOwnership{}, fmt.Errorf("helper ownership is unavailable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxHelperOwnershipBytes {
		return helperOwnership{}, errors.New("helper ownership record is not a supported regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return helperOwnership{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return helperOwnership{}, errors.New("helper ownership record changed before reading")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxHelperOwnershipBytes+1))
	if err != nil {
		return helperOwnership{}, err
	}
	if len(data) > maxHelperOwnershipBytes {
		return helperOwnership{}, errors.New("helper ownership record exceeds its size limit")
	}
	var fields map[string]json.RawMessage
	if err := fileutil.DecodeJSONStrict(data, &fields); err != nil {
		return helperOwnership{}, err
	}
	if len(fields) != 6 {
		return helperOwnership{}, errors.New("helper ownership record is incomplete")
	}
	for _, key := range []string{"version", "target", "helperName", "helperHash", "updateHash", "phase"} {
		if value, exists := fields[key]; !exists || string(value) == "null" {
			return helperOwnership{}, fmt.Errorf("helper ownership has no explicit %s", key)
		}
	}
	var record helperOwnership
	if err := fileutil.DecodeJSONStrict(data, &record); err != nil {
		return helperOwnership{}, err
	}
	if record.Version != helperOwnershipVersion || record.Target != target || record.HelperName == "" || filepath.Base(record.HelperName) != record.HelperName || record.HelperName == "." || record.HelperName == ".." || record.HelperName == helperOwnershipFile || record.HelperName == readyFileName ||
		!hashPattern.MatchString(record.HelperHash) || !hashPattern.MatchString(record.UpdateHash) || (record.Phase != helperPhasePrepared && record.Phase != helperPhaseCleaning) {
		return helperOwnership{}, errors.New("helper ownership does not match this update")
	}
	return record, nil
}

// cleanupRecordedHelper removes only verified files, preserving unknown directories.
// The cleaning checkpoint is kept until owned payloads have been removed.
func cleanupRecordedHelper(directory, target string) error {
	if !isHelperDirectory(directory, target) {
		return errors.New("helper directory does not match the executable")
	}
	directoryInfo, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("helper directory is linked or nonregular")
	}
	record, err := readHelperOwnership(directory, target)
	if err != nil {
		return err
	}
	if _, err := inspectOwnedHelperFile(target, record.UpdateHash); err != nil {
		return fmt.Errorf("verify installed application before helper cleanup: %w", err)
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		return err
	}
	openedDirectory, statErr := directoryFile.Stat()
	if statErr != nil || !openedDirectory.IsDir() || !os.SameFile(directoryInfo, openedDirectory) {
		return errors.Join(errors.New("helper directory changed before inspection"), statErr, directoryFile.Close())
	}
	const ownedFileCount = 3 // One helper, readiness marker and ownership record.
	entries, readErr := directoryFile.ReadDir(ownedFileCount + 1)
	closeErr := directoryFile.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) > ownedFileCount {
		return errors.New("helper directory contains unrecorded entries")
	}
	for _, entry := range entries {
		if name := entry.Name(); name != record.HelperName && name != readyFileName && name != helperOwnershipFile {
			return fmt.Errorf("unknown helper directory entry retained: %s", name)
		}
	}
	readyDigest := sha256.Sum256([]byte(record.UpdateHash))
	owned := map[string]string{record.HelperName: record.HelperHash, readyFileName: hex.EncodeToString(readyDigest[:])}
	observed := make(map[string]os.FileInfo, len(owned))
	for name, digest := range owned {
		info, err := inspectOwnedHelperFile(filepath.Join(directory, name), digest)
		if errors.Is(err, os.ErrNotExist) && record.Phase == helperPhaseCleaning {
			continue
		}
		if err != nil {
			return err
		}
		observed[name] = info
	}
	record.Phase = helperPhaseCleaning
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(directory, helperOwnershipFile), data, 0o600); err != nil {
		return err
	}
	for name, info := range observed {
		path := filepath.Join(directory, name)
		current, err := inspectOwnedHelperFile(path, owned[name])
		if err != nil || !os.SameFile(info, current) {
			return fmt.Errorf("owned helper file changed before removal: %s", name)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	currentDirectory, err := os.Lstat(directory)
	if err != nil || !os.SameFile(directoryInfo, currentDirectory) {
		return errors.New("helper directory changed during cleanup")
	}
	if _, err := inspectOwnedHelperFile(filepath.Join(directory, helperOwnershipFile), helperBytesHash(data)); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(directory, helperOwnershipFile)); err != nil {
		return err
	}
	return os.Remove(directory)
}

func inspectOwnedHelperFile(path, digest string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("owned helper file is linked or nonregular: %s", path)
	}
	actual, err := fileHash(path)
	if err != nil {
		return nil, err
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, current) || actual != digest {
		return nil, fmt.Errorf("owned helper file identity or checksum changed: %s", path)
	}
	return info, nil
}

func helperBytesHash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
