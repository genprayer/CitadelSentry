package game

import (
	"os"
	"runtime"
)

func writableConfigMode(mode os.FileMode) os.FileMode {
	if runtime.GOOS == "windows" {
		// Windows exposes its read-only attribute through all three write bits.
		return mode | 0o222
	}
	return mode | 0o200
}

func restoredConfigMode(entry BackupEntry, currentMode os.FileMode) os.FileMode {
	if entry.OriginalMode == nil {
		// Historical records do not describe original attributes. Keep the
		// actual current permissions instead of claiming to reconstruct them.
		return currentMode
	}
	originalMode := os.FileMode(*entry.OriginalMode)
	if currentMode != originalMode && currentMode != writableConfigMode(originalMode) {
		// Preserve permission changes made outside the transaction.
		return currentMode
	}
	return originalMode
}
