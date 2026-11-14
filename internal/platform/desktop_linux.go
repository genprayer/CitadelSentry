//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const desktopLaunchTimeout = 8 * time.Second

func runDesktopLauncher(parent context.Context, resource string) error {
	return runExternalLauncher(parent, "xdg-open", resource)
}

func runSteamLauncher(parent context.Context, uri string) error {
	executable, err := exec.LookPath("steam")
	if errors.Is(err, exec.ErrNotFound) {
		return runDesktopLauncher(parent, uri)
	}
	if err != nil {
		return fmt.Errorf("find Steam launcher: %w", err)
	}
	// Steam's CLI dispatches to its running client without the desktop URI handler.
	// A failed dispatch is not retried through another handler: it may have sent it.
	return runExternalLauncher(parent, executable, uri)
}

func runExternalLauncher(parent context.Context, executable, resource string) error {
	if resource == "" {
		return fmt.Errorf("desktop resource is required")
	}
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, desktopLaunchTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, resource)
	command.WaitDelay = time.Second
	return waitForDesktopCommand(ctx, command)
}

func waitForDesktopCommand(ctx context.Context, command *exec.Cmd) error {
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("desktop launcher: %w", ctx.Err())
		}
		return fmt.Errorf("desktop launcher: %w", err)
	}
	return nil
}
