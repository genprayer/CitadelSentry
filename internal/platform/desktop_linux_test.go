//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func installTestDesktopLauncher(t *testing.T, primitive string) {
	t.Helper()
	path, err := exec.LookPath(primitive)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "xdg-open"), data, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
}

func TestLinuxDesktopLauncherReportsItsActualExitStatus(t *testing.T) {
	installTestDesktopLauncher(t, "false")
	if err := NewAdapter().OpenBrowser("https://example.invalid/"); err == nil {
		t.Fatal("failed desktop dispatch was reported as success")
	}
}

func TestLinuxDesktopLauncherWaitsForAcknowledgement(t *testing.T) {
	installTestDesktopLauncher(t, "true")
	if err := NewAdapter().LaunchURI(context.Background(), "steam://joinlobby/393380/1"); err != nil {
		t.Fatal(err)
	}
}

func installRecordedLauncher(t *testing.T, directory, name string, exitCode int) {
	t.Helper()
	data := "#!/bin/sh\nprintf '%s\\n' '" + name + "' \"$@\" >> \"$CITADEL_TEST_LAUNCH_CAPTURE\"\nexit " + fmt.Sprint(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxSteamDispatchBypassesDesktopHandlerAndPreservesOneURIArgument(t *testing.T) {
	directory := t.TempDir()
	capture := filepath.Join(directory, "dispatch")
	t.Setenv("PATH", directory)
	t.Setenv("CITADEL_TEST_LAUNCH_CAPTURE", capture)
	installRecordedLauncher(t, directory, "steam", 0)
	installRecordedLauncher(t, directory, "xdg-open", 0)
	const uri = "steam://joinlobby/393380/109775242037177537/76561199185206154"
	if err := NewAdapter().LaunchURI(context.Background(), uri); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(capture)
	if err != nil || string(data) != "steam\n"+uri+"\n" {
		t.Fatalf("unexpected native dispatch: %q, %v", data, err)
	}
}

func TestLinuxFailedSteamDispatchDoesNotSendDuplicateThroughDesktopHandler(t *testing.T) {
	directory := t.TempDir()
	capture := filepath.Join(directory, "dispatch")
	t.Setenv("PATH", directory)
	t.Setenv("CITADEL_TEST_LAUNCH_CAPTURE", capture)
	installRecordedLauncher(t, directory, "steam", 42)
	installRecordedLauncher(t, directory, "xdg-open", 0)
	const uri = "steam://joinlobby/393380/1"
	var exitError *exec.ExitError
	if err := NewAdapter().LaunchURI(context.Background(), uri); !errors.As(err, &exitError) || exitError.ExitCode() != 42 {
		t.Fatalf("Steam launcher error disappeared: %v", err)
	}
	data, err := os.ReadFile(capture)
	if err != nil || string(data) != "steam\n"+uri+"\n" {
		t.Fatalf("failed Steam launch caused another dispatch: %q, %v", data, err)
	}
}

func TestLinuxDesktopLauncherHonorsCancellationAndReapsItsOwnedCommand(t *testing.T) {
	installTestDesktopLauncher(t, "sleep")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	command := exec.CommandContext(ctx, "xdg-open", "60")
	if err := waitForDesktopCommand(ctx, command); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("launch cancellation disappeared: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancelled launch did not finish command cleanup")
	}
	if command.Process == nil || command.ProcessState == nil || command.ProcessState.Pid() != command.Process.Pid {
		t.Fatal("owned launcher process was not reaped")
	}
}

func TestLinuxFolderOpenRejectsMissingTargetBeforeDispatch(t *testing.T) {
	installTestDesktopLauncher(t, "true")
	if err := NewAdapter().OpenFolder(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing folder was treated as opened: %v", err)
	}
}
