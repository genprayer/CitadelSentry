//go:build windows

package fileutil

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

func replaceFile(source string, destination string) error {
	destPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	attrs, getErr := windows.GetFileAttributes(destPtr)
	if getErr != nil && !errors.Is(getErr, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(getErr, windows.ERROR_PATH_NOT_FOUND) {
		return fmt.Errorf("inspect destination attributes: %w", getErr)
	}
	hasReadOnlyAttribute := getErr == nil && attrs&windows.FILE_ATTRIBUTE_READONLY != 0
	if hasReadOnlyAttribute {
		if err := windows.SetFileAttributes(destPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY); err != nil {
			return fmt.Errorf("make destination writable: %w", err)
		}
	}
	err = moveFile(source, destination, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	if err != nil && hasReadOnlyAttribute {
		if restoreErr := windows.SetFileAttributes(destPtr, attrs); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore destination attributes after failed replacement: %w", restoreErr))
		}
	}
	return err
}

func createFile(source string, destination string) error {
	// Omitting REPLACE_EXISTING preserves a file created after the absence check.
	return moveFile(source, destination, windows.MOVEFILE_WRITE_THROUGH)
}

func moveFile(source string, destination string, flags uint32) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		sourcePtr,
		destinationPtr,
		flags,
	)
}

// SetReadOnly sets or removes the FILE_ATTRIBUTE_READONLY attribute on Windows.
func SetReadOnly(path string, readOnly bool) error {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	if readOnly {
		attrs |= windows.FILE_ATTRIBUTE_READONLY
	} else {
		attrs &^= windows.FILE_ATTRIBUTE_READONLY
	}
	return windows.SetFileAttributes(ptr, attrs)
}
