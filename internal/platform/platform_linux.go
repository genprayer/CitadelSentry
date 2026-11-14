//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const squadProcessPattern = `(^|.*[/\\])(SquadGame|SquadGame-Win64-Shipping)\.exe([[:space:]]|$)`

type linuxAdapter struct{}

func NewAdapter() Adapter {
	return &linuxAdapter{}
}

func (a *linuxAdapter) Name() string { return "linux" }

func (a *linuxAdapter) DetectGPU() GPUInfo { return DetectGPU() }

func (a *linuxAdapter) CheckSquadRunning() (bool, error) {
	return checkProcessCommand(exec.Command("pgrep", "-f", squadProcessPattern))
}

func (a *linuxAdapter) CheckSteamRunning() (bool, error) {
	return checkProcessCommand(exec.Command("pgrep", "-x", "steam"))
}

func checkProcessCommand(command *exec.Cmd) (bool, error) {
	out, err := command.Output()
	if err == nil {
		return strings.TrimSpace(string(out)) != "", nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("inspect processes: %w", err)
}

func (a *linuxAdapter) OpenBrowser(url string) error {
	return runDesktopLauncher(context.Background(), url)
}

func (a *linuxAdapter) OpenFolder(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect folder target: %w", err)
	}
	target := path
	if !info.IsDir() {
		target = filepath.Dir(path)
	}
	return runDesktopLauncher(context.Background(), target)
}

// LaunchURI opens a protocol URI (steam://joinlobby/..., steam://connect/...) on Linux.
func (a *linuxAdapter) LaunchURI(ctx context.Context, uri string) error {
	return runSteamLauncher(ctx, uri)
}

// CloseGame terminates the running game process on Linux.
func (a *linuxAdapter) CloseGame() error {
	isRunning, err := a.CheckSquadRunning()
	if err != nil {
		return err
	}
	if !isRunning {
		return nil
	}
	return exec.Command("pkill", "-f", squadProcessPattern).Run()
}

func (a *linuxAdapter) ExecutePowerAction(action string) error {
	switch action {
	case "sleep":
		return exec.Command("systemctl", "suspend").Run()
	case "shutdown":
		return exec.Command("systemctl", "poweroff").Run()
	default:
		return fmt.Errorf("unsupported power action: %s", action)
	}
}

func (a *linuxAdapter) CancelPowerAction() error {
	return exec.Command("shutdown", "-c").Run()
}

// CheckProcessPrioritySupport rejects unsupported tuning before game files are patched.
func (a *linuxAdapter) CheckProcessPrioritySupport() error {
	return fmt.Errorf("low CPU priority is unavailable on Linux; disable this option before starting Potato Mode")
}

// SetProcessLowPriority reports unavailable tuning instead of claiming a successful no-op.
func (a *linuxAdapter) SetProcessLowPriority(processNames ...string) error {
	return a.CheckProcessPrioritySupport()
}

// RestoreProcessPriority checks recovery metadata without interpreting Windows classes as Linux values.
func (a *linuxAdapter) RestoreProcessPriority() error {
	path, err := priorityBackupPath()
	if err != nil {
		return err
	}
	backup, err := readPriorityBackup(path)
	if err != nil {
		return err
	}
	if len(backup.Entries) != 0 {
		return fmt.Errorf("pending Windows process priority backup cannot be restored on Linux")
	}
	return nil
}
