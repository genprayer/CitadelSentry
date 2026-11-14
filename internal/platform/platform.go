package platform

import "context"

// ReleaseFunc releases a system resource or sleep inhibition lock.
type ReleaseFunc func()

// Adapter abstracts operating-system specific functionality.
// All platform-specific behaviour lives here — game.go must never use runtime.GOOS.
type Adapter interface {
	Name() string

	// Process detection
	// Checks return an error when process state cannot be determined.
	CheckSquadRunning() (bool, error)
	CheckSteamRunning() (bool, error)

	// Power management
	PreventSleep(ctx context.Context, reason string) (ReleaseFunc, error)
	ExecutePowerAction(action string) error // "sleep", "shutdown"
	CancelPowerAction() error

	// External resources
	OpenBrowser(url string) error
	OpenFolder(path string) error

	// Game lifecycle — the canonical DI path for all game operations
	LaunchURI(ctx context.Context, uri string) error // e.g. steam://joinlobby/..., steam://connect/...
	CloseGame() error                                // terminates the running game process

	// Process performance tuning
	CheckProcessPrioritySupport() error
	SetProcessLowPriority(processNames ...string) error
	RestoreProcessPriority() error

	// Hardware detection
	DetectGPU() GPUInfo
}
