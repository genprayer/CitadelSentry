package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"citadel-seeder/internal/config"
	"citadel-seeder/internal/fileutil"
)

const (
	priorityBackupVersion  = 1
	maxPriorityBackupBytes = 64 * 1024
	priorityClassIdle      = 0x00000040
)

// ErrPriorityProcessPending means no supported game process is available yet.
var ErrPriorityProcessPending = errors.New("supported Squad game process is not available yet")

// ErrPriorityAccessDenied means anti-cheat or Windows permissions restricted priority adjustments.
var ErrPriorityAccessDenied = errors.New("access denied by anti-cheat or OS permissions")

type priorityIdentity struct {
	ProcessID    uint32 `json:"processId"`
	CreationTime uint64 `json:"creationTime"`
	Name         string `json:"name"`
}

type priorityEntry struct {
	Identity priorityIdentity `json:"identity"`
	Original uint32           `json:"original"`
	Applied  uint32           `json:"applied"`
}

type priorityBackup struct {
	Version  int             `json:"version"`
	Entries  []priorityEntry `json:"entries"`
	Checksum string          `json:"sha256"`
}

type priorityTarget interface {
	identity() priorityIdentity
	currentPriority() (uint32, error)
	setPriority(uint32) error
	close() error
}

type priorityInspector func(priorityIdentity) (priorityTarget, bool, error)

func priorityBackupPath() (string, error) {
	directory, err := config.StateDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "process-priority.json"), nil
}

func priorityChecksum(entries []priorityEntry) (string, error) {
	data, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func isPriorityClass(value uint32) bool {
	switch value {
	case 0x20, priorityClassIdle, 0x80, 0x100, 0x4000, 0x8000:
		return true
	default:
		return false
	}
}

func readPriorityBackup(path string) (priorityBackup, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return priorityBackup{Version: priorityBackupVersion}, nil
	}
	if err != nil {
		return priorityBackup{}, fmt.Errorf("read process priority backup: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPriorityBackupBytes+1))
	if err != nil || len(data) > maxPriorityBackupBytes {
		return priorityBackup{}, fmt.Errorf("process priority backup is unreadable or too large: %v", err)
	}
	var backup priorityBackup
	if err := fileutil.DecodeJSONStrict(data, &backup); err != nil {
		return priorityBackup{}, fmt.Errorf("parse process priority backup: %w", err)
	}
	expected, err := priorityChecksum(backup.Entries)
	if err != nil || backup.Version != priorityBackupVersion || backup.Checksum != expected {
		return priorityBackup{}, fmt.Errorf("process priority backup version or checksum is invalid")
	}
	seen := make(map[priorityIdentity]bool)
	for _, entry := range backup.Entries {
		if entry.Identity.ProcessID == 0 || entry.Identity.CreationTime == 0 || entry.Identity.Name == "" ||
			seen[entry.Identity] || !isPriorityClass(entry.Original) || entry.Applied != priorityClassIdle {
			return priorityBackup{}, fmt.Errorf("invalid or ambiguous process priority backup entry")
		}
		seen[entry.Identity] = true
	}
	return backup, nil
}

func writePriorityBackup(path string, backup priorityBackup) error {
	checksum, err := priorityChecksum(backup.Entries)
	if err != nil {
		return err
	}
	backup.Version = priorityBackupVersion
	backup.Checksum = checksum
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxPriorityBackupBytes {
		return fmt.Errorf("process priority backup exceeds its size limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, data, 0o600)
}

func restorePriorityBackup(path string, inspect priorityInspector) error {
	backup, err := readPriorityBackup(path)
	if err != nil || len(backup.Entries) == 0 {
		return err
	}
	remaining := make([]priorityEntry, 0, len(backup.Entries))
	var failures []error
	for _, entry := range backup.Entries {
		target, exists, err := inspect(entry.Identity)
		if err != nil {
			remaining = append(remaining, entry)
			failures = append(failures, err)
			continue
		}
		if !exists {
			continue
		}
		if target == nil {
			remaining = append(remaining, entry)
			failures = append(failures, fmt.Errorf("priority process inspection returned no target"))
			continue
		}
		if target.identity() != entry.Identity {
			actual := target.identity()
			if actual.ProcessID == entry.Identity.ProcessID && actual.CreationTime == entry.Identity.CreationTime {
				remaining = append(remaining, entry)
				failures = append(failures, fmt.Errorf("recorded process image could not be verified"))
			}
			if err := target.close(); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		current, restoreErr := target.currentPriority()
		if restoreErr != nil {
			restoreErr = fmt.Errorf("inspect priority process %d: %w", entry.Identity.ProcessID, restoreErr)
		}
		if restoreErr == nil && !isPriorityClass(current) {
			restoreErr = fmt.Errorf("process priority could not be verified during recovery")
		}
		if restoreErr == nil && current == entry.Applied {
			restoreErr = target.setPriority(entry.Original)
			if restoreErr == nil {
				var restored uint32
				restored, restoreErr = target.currentPriority()
				if restoreErr == nil && restored != entry.Original {
					restoreErr = fmt.Errorf("process %d did not restore its recorded priority", entry.Identity.ProcessID)
				}
			}
		}
		if restoreErr != nil {
			remaining = append(remaining, entry)
			failures = append(failures, restoreErr)
		}
		if err := target.close(); err != nil {
			failures = append(failures, err)
		}
	}
	backup.Entries = remaining
	if err := writePriorityBackup(path, backup); err != nil {
		failures = append(failures, fmt.Errorf("persist pending priority recovery: %w", err))
	}
	return errors.Join(failures...)
}

func applyPriorityTargets(path string, targets []priorityTarget, inspect priorityInspector) (result error) {
	defer func() {
		for _, target := range targets {
			result = errors.Join(result, target.close())
		}
	}()
	if len(targets) == 0 {
		return ErrPriorityProcessPending
	}
	backup, err := readPriorityBackup(path)
	if err != nil {
		return err
	}
	tracked := make(map[priorityIdentity]priorityEntry)
	for _, entry := range backup.Entries {
		tracked[entry.Identity] = entry
	}
	type change struct {
		target   priorityTarget
		original uint32
	}
	var changes []change
	seenTargets := make(map[priorityIdentity]bool)
	for _, target := range targets {
		identity := target.identity()
		if identity.ProcessID == 0 || identity.CreationTime == 0 || identity.Name == "" || seenTargets[identity] {
			return fmt.Errorf("process priority target identity is invalid or duplicated")
		}
		seenTargets[identity] = true
		original, err := target.currentPriority()
		if err != nil || !isPriorityClass(original) {
			return fmt.Errorf("verify original priority for process %d: %v", target.identity().ProcessID, err)
		}
		if entry, exists := tracked[target.identity()]; exists {
			if original != entry.Applied {
				return fmt.Errorf("process %d priority changed outside Sentry", target.identity().ProcessID)
			}
			continue
		}
		if original == priorityClassIdle {
			continue
		}
		if original == 0x100 {
			return fmt.Errorf("cannot guarantee restoration of realtime process priority")
		}
		backup.Entries = append(backup.Entries, priorityEntry{Identity: target.identity(), Original: original, Applied: priorityClassIdle})
		changes = append(changes, change{target: target, original: original})
	}
	if len(changes) == 0 {
		return nil
	}
	if err := writePriorityBackup(path, backup); err != nil {
		return fmt.Errorf("save original process priorities before changing them: %w", err)
	}
	for _, change := range changes {
		current, err := change.target.currentPriority()
		if err == nil && current != change.original {
			err = fmt.Errorf("process priority changed before application")
		}
		if err == nil {
			err = change.target.setPriority(priorityClassIdle)
		}
		if err == nil {
			current, err = change.target.currentPriority()
			if err == nil && current != priorityClassIdle {
				err = fmt.Errorf("process priority change was not confirmed")
			}
		}
		if err != nil {
			return errors.Join(err, restorePriorityBackup(path, inspect))
		}
	}
	return nil
}
