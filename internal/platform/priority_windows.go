//go:build windows

package platform

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsPriorityTarget struct {
	handle  windows.Handle
	process priorityIdentity
}

// Launchers are excluded because Windows inherits their priority in child processes.
var squadPriorityProcessNames = [...]string{"SquadGame.exe", "SquadGame-Win64-Shipping.exe"}

func (t *windowsPriorityTarget) identity() priorityIdentity { return t.process }
func (t *windowsPriorityTarget) currentPriority() (uint32, error) {
	return windows.GetPriorityClass(t.handle)
}
func (t *windowsPriorityTarget) setPriority(priority uint32) error {
	err := windows.SetPriorityClass(t.handle, priority)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return fmt.Errorf("set process priority: %w", ErrPriorityAccessDenied)
	}
	return err
}
func (t *windowsPriorityTarget) close() error { return windows.CloseHandle(t.handle) }

func openWindowsPriorityTarget(processID uint32) (*windowsPriorityTarget, bool, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, processID)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil, false, nil
	}
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil, false, fmt.Errorf("open priority process %d: %w", processID, ErrPriorityAccessDenied)
	}
	if err != nil {
		return nil, false, fmt.Errorf("open priority process %d: %w", processID, err)
	}
	isOwned := false
	defer func() {
		if !isOwned {
			windows.CloseHandle(handle)
		}
	}()
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return nil, false, err
	}
	if state == windows.WAIT_OBJECT_0 {
		return nil, false, nil
	}
	if state != uint32(windows.WAIT_TIMEOUT) {
		return nil, false, fmt.Errorf("unknown process wait state %d", state)
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return nil, false, err
	}
	imagePath := make([]uint16, windows.MAX_LONG_PATH)
	imageLength := uint32(len(imagePath))
	if err := windows.QueryFullProcessImageName(handle, 0, &imagePath[0], &imageLength); err != nil {
		return nil, false, err
	}
	name := strings.ToLower(filepath.Base(windows.UTF16ToString(imagePath[:imageLength])))
	creationTime := uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime)
	if creationTime == 0 || name == "" {
		return nil, false, fmt.Errorf("process identity could not be verified")
	}
	isOwned = true
	return &windowsPriorityTarget{handle: handle, process: priorityIdentity{ProcessID: processID, CreationTime: creationTime, Name: name}}, true, nil
}

func inspectWindowsPriority(identity priorityIdentity) (priorityTarget, bool, error) {
	if !isSquadPriorityName(identity.Name) {
		return nil, false, fmt.Errorf("priority backup targets an unsupported process")
	}
	target, exists, err := openWindowsPriorityTarget(identity.ProcessID)
	if err != nil || !exists {
		return nil, false, err
	}
	if target.identity() != identity {
		actual := target.identity()
		closeError := target.close()
		if actual.ProcessID == identity.ProcessID && actual.CreationTime == identity.CreationTime {
			return nil, false, errors.Join(fmt.Errorf("recorded process image could not be verified"), closeError)
		}
		return nil, false, closeError
	}
	return target, true, nil
}

func isSquadPriorityName(name string) bool {
	for _, supported := range squadPriorityProcessNames {
		if strings.ToLower(name) == strings.ToLower(supported) {
			return true
		}
	}
	return false
}

// CheckProcessPrioritySupport reports Windows API support; target rights are checked per process.
func (a *windowsAdapter) CheckProcessPrioritySupport() error { return nil }

// SetProcessLowPriority checkpoints verified game priorities before applying the selected class.
func (a *windowsAdapter) SetProcessLowPriority(processNames ...string) error {
	a.priorityMu.Lock()
	defer a.priorityMu.Unlock()
	if len(processNames) == 0 {
		processNames = squadPriorityProcessNames[:]
	}
	for _, name := range processNames {
		if !isSquadPriorityName(name) {
			return fmt.Errorf("unsupported priority process name: %s", name)
		}
	}
	path, err := priorityBackupPath()
	if err != nil {
		return err
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return err
	}
	var targets []priorityTarget
	isTransferred := false
	defer func() {
		if !isTransferred {
			for _, target := range targets {
				target.close()
			}
		}
	}()
	for {
		name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		for _, processName := range processNames {
			if strings.ToLower(processName) != name {
				continue
			}
			target, exists, err := openWindowsPriorityTarget(entry.ProcessID)
			if err != nil {
				return err
			}
			if exists {
				targets = append(targets, target)
				if target.identity().Name != name {
					return fmt.Errorf("process identity changed during priority enumeration")
				}
			}
			break
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return err
			}
			break
		}
	}
	isTransferred = true
	return applyPriorityTargets(path, targets, inspectWindowsPriority)
}

// RestoreProcessPriority retries verified originals without changing another process instance.
func (a *windowsAdapter) RestoreProcessPriority() error {
	a.priorityMu.Lock()
	defer a.priorityMu.Unlock()
	path, err := priorityBackupPath()
	if err != nil {
		return err
	}
	return restorePriorityBackup(path, inspectWindowsPriority)
}
