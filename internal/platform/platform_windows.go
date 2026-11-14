//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	setThreadExecutionStateProc = kernel32.NewProc("SetThreadExecutionState")
)

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

type windowsAdapter struct{ priorityMu sync.Mutex }

var squadProcessNames = [...]string{
	"SquadGame.exe",
	"SquadGame-Win64-Shipping.exe",
	"Squad.exe",
	"Squad_Launcher.exe",
}

func NewAdapter() Adapter {
	return &windowsAdapter{}
}

func (a *windowsAdapter) Name() string { return "windows" }

func (a *windowsAdapter) DetectGPU() GPUInfo { return DetectGPU() }

func checkProcessRunning(targetName string) (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return false, nil
		}
		return false, fmt.Errorf("read process snapshot: %w", err)
	}

	targetLower := strings.ToLower(targetName)
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.ToLower(name) == targetLower {
			return true, nil
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return false, nil
			}
			return false, fmt.Errorf("continue process snapshot: %w", err)
		}
	}
}

func (a *windowsAdapter) CheckSquadRunning() (bool, error) {
	for _, processName := range squadProcessNames {
		isRunning, err := checkProcessRunning(processName)
		if err != nil {
			return false, err
		}
		if isRunning {
			return true, nil
		}
	}
	return false, nil
}

func (a *windowsAdapter) CheckSteamRunning() (bool, error) { return checkProcessRunning("steam.exe") }

func (a *windowsAdapter) PreventSleep(ctx context.Context, reason string) (ReleaseFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var previous uintptr
	release, err := acquireThreadResource(func() error {
		result, _, callErr := setThreadExecutionStateProc.Call(uintptr(esContinuous | esSystemRequired))
		if result == 0 {
			return fmt.Errorf("set thread execution state: %w", callErr)
		}
		previous = result
		return nil
	}, func() error {
		result, _, callErr := setThreadExecutionStateProc.Call(previous | uintptr(esContinuous))
		if result == 0 {
			return fmt.Errorf("restore thread execution state: %w", callErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func (a *windowsAdapter) OpenBrowser(url string) error {
	return openWindowsResource(context.Background(), url)
}

func (a *windowsAdapter) OpenFolder(path string) error {
	if path == "" {
		return fmt.Errorf("folder target is required")
	}
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("inspect folder target: %w", err)
	}
	if !info.IsDir() {
		cleanPath = filepath.Dir(cleanPath)
	}
	return openWindowsResource(context.Background(), cleanPath)
}

// LaunchURI opens a protocol URI through the Windows shell without a command interpreter.
func (a *windowsAdapter) LaunchURI(ctx context.Context, uri string) error {
	return openWindowsResource(ctx, uri)
}

func openWindowsResource(ctx context.Context, resource string) error {
	if resource == "" {
		return fmt.Errorf("shell resource is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("encode verb: %w", err)
	}
	file, err := windows.UTF16PtrFromString(resource)
	if err != nil {
		return fmt.Errorf("encode shell resource: %w", err)
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("open shell resource: %w", err)
	}
	return nil
}

// CloseGame terminates the running Squad process.
func (a *windowsAdapter) CloseGame() error {
	isRunning, err := a.CheckSquadRunning()
	if err != nil {
		return err
	}
	if !isRunning {
		return nil
	}
	for _, processName := range squadProcessNames {
		isRunning, err := checkProcessRunning(processName)
		if err != nil {
			return err
		}
		if !isRunning {
			continue
		}
		cmd := exec.Command("taskkill", "/f", "/t", "/im", processName)
		HideWindow(cmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("terminate %s: %w", processName, err)
		}
	}
	return nil
}

func (a *windowsAdapter) ExecutePowerAction(action string) error {
	switch action {
	case "sleep":
		cmd := exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
		HideWindow(cmd)
		return cmd.Run()
	case "shutdown":
		cmd := exec.Command("shutdown", "/s", "/t", "15", "/c", "Citadel Sentry: target reached")
		HideWindow(cmd)
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported power action: %s", action)
	}
}

// CancelPowerAction aborts any pending system shutdown in Windows.
func (a *windowsAdapter) CancelPowerAction() error {
	cmd := exec.Command("shutdown", "/a")
	HideWindow(cmd)
	return cmd.Run()
}
