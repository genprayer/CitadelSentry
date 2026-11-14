package game

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/logger"
	"citadel-seeder/internal/platform"
)

const (
	defaultProcessPollInterval = 250 * time.Millisecond
	defaultServerSwitchTimeout = 30 * time.Second
	defaultStartupTimeout      = 12 * time.Minute
	defaultJoinJitterMax       = 3500 * time.Millisecond
	playerPresencePollInterval = 15 * time.Second
	playerPresenceTimeout      = 8 * time.Second
)

// Controller orchestrates Squad launch and connect automation.
// All platform-specific calls go through the platform.Adapter — no runtime.GOOS checks.
type Controller struct {
	mu                  sync.Mutex
	connection          *connectionLog
	logPaths            func() []string
	startupTimeout      time.Duration
	joinJitterMax       time.Duration
	platform            platform.Adapter
	processPollInterval time.Duration
	serverSwitchTimeout time.Duration
	verifyPlayer        func(context.Context, string, string) (bool, error)
	lastPlayerCheck     time.Time
	playerConfirmed     bool
	connectionContext   context.Context
}

// SetPlayerVerifier supplies the existing public roster contract for APIs that
// intentionally hide game endpoints. Configure it before starting any mission.
func (c *Controller) SetPlayerVerifier(verify func(context.Context, string, string) (bool, error)) {
	c.verifyPlayer = verify
}

// SetJoinJitterMax sets the maximum random jitter before joining to prevent EOS spikes.
func (c *Controller) SetJoinJitterMax(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.joinJitterMax = d
}

// SetLogPaths overrides log discovery for testing or custom environments.
func (c *Controller) SetLogPaths(paths func() []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logPaths = paths
}

func NewController(plt platform.Adapter) *Controller {
	return &Controller{
		platform:            plt,
		processPollInterval: defaultProcessPollInterval,
		serverSwitchTimeout: defaultServerSwitchTimeout,
		startupTimeout:      defaultStartupTimeout,
		logPaths:            squadLogPaths,
	}
}

// Connect opens a validated Squad steam://joinlobby URL through the OS adapter.
func (c *Controller) Connect(ctx context.Context, joinURL string) error {
	return c.connectResolved(ctx, func(context.Context) (string, error) { return joinURL, nil })
}

// ConnectResolved resolves the current invitation after any required startup.
func (c *Controller) ConnectResolved(ctx context.Context, resolve func(context.Context) (string, error), shouldSwitch bool) error {
	if shouldSwitch {
		return c.switchResolved(ctx, resolve)
	}
	return c.connectResolved(ctx, resolve)
}

func (c *Controller) connectResolved(ctx context.Context, resolve func(context.Context) (string, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	observer, err := newConnectionLog(c.logPaths())
	if err != nil {
		return err
	}
	c.connection = observer
	c.connectionContext = ctx
	c.lastPlayerCheck = time.Time{}
	c.playerConfirmed = false
	isRunning, err := c.CheckSquadRunning()
	if err != nil {
		return err
	}
	if !isRunning {
		// Startup invitations can be discarded while EOS/intro is initializing.
		// Launch first and dispatch the lobby only after the menu and online login.
		launchURI := "steam://rungameid/" + squadSteamAppID
		if err := c.platform.LaunchURI(ctx, launchURI); err != nil {
			return fmt.Errorf("start Squad: %w", err)
		}
		readyCtx, cancel := context.WithTimeout(ctx, c.startupTimeout)
		defer cancel()
		if err := c.waitForClientReady(readyCtx, observer); err != nil {
			return err
		}
		observer, err = newConnectionLog(c.logPaths())
		if err != nil {
			return err
		}
		c.connection = observer
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	joinURL, err := resolve(ctx)
	if err != nil {
		return fmt.Errorf("resolve invitation after Squad startup: %w", err)
	}
	if c.joinJitterMax > 0 {
		jitter := time.Duration(rand.Int63n(int64(c.joinJitterMax)))
		logger.Info("Staggered launch: applying randomized jitter before join", "delayMs", jitter.Milliseconds())
		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	logger.Info("Launching Squad join URI through platform adapter", "joinURL", joinURL)
	err = c.platform.LaunchURI(ctx, joinURL)
	if err != nil {
		logger.Error("Failed to launch Squad join URI", "joinURL", joinURL, "error", err)
	}
	return err
}

func (c *Controller) waitForClientReady(ctx context.Context, observer *connectionLog) error {
	ticker := time.NewTicker(c.processPollInterval)
	defer ticker.Stop()
	hasObservedProcess := false
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait for Squad menu and online login: %w", err)
		}
		isRunning, err := c.CheckSquadRunning()
		if err != nil {
			return err
		}
		if !isRunning && hasObservedProcess {
			return fmt.Errorf("Squad closed before its menu and online login were ready")
		}
		hasObservedProcess = hasObservedProcess || isRunning
		if err := observer.poll(); err != nil {
			return err
		}
		if isRunning && observer.isCaughtUp && observer.state.isMenuLoaded && observer.state.isAuthenticated {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Squad menu and online login: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// CheckServerConnection requires fresh welcome and deployment-menu records for
// the requested endpoint or exact player roster, and observes disconnections.
func (c *Controller) CheckServerConnection(server citadel.Server) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connection == nil {
		return false, fmt.Errorf("Squad connection observation has not started")
	}
	if err := c.connection.poll(); err != nil {
		return false, err
	}
	if !c.connection.isCaughtUp {
		return false, nil
	}
	if c.connection.state.failure != nil {
		return false, c.connection.state.failure
	}
	if c.connection.state.wasSteamInviteBusy {
		return false, ErrSteamInviteBusy
	}
	if net.ParseIP(server.ServerIP) != nil && server.GamePort > 0 {
		return c.connection.state.matches(server), nil
	}

	// Check against known authoritative Citadel server endpoint
	if knownIP, knownPort, ok := GetCitadelServerEndpoint(server.ID); ok {
		if c.connection.state.isConnected && c.connection.state.endpoint == net.JoinHostPort(knownIP, fmt.Sprintf("%d", knownPort)) {
			return true, nil
		}
	}

	if c.verifyPlayer == nil {
		return false, fmt.Errorf("server endpoint is hidden and player-roster confirmation is not configured")
	}
	if !c.connection.state.isConnected || c.connection.state.localSteamID == "" {
		c.playerConfirmed = false
		return false, nil
	}
	if time.Since(c.lastPlayerCheck) < playerPresencePollInterval {
		return c.playerConfirmed, nil
	}
	ctx, cancel := context.WithTimeout(c.connectionContext, playerPresenceTimeout)
	defer cancel()
	confirmed, err := c.verifyPlayer(ctx, server.ID, c.connection.state.localSteamID)
	c.lastPlayerCheck = time.Now()
	c.playerConfirmed = confirmed && err == nil
	return c.playerConfirmed, err
}

// GetObservedConnectedServerID returns the server ID if the client is currently connected
// to any known Citadel server, or "" if not connected to a Citadel server.
func (c *Controller) GetObservedConnectedServerID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connection == nil || !c.connection.state.isConnected {
		return ""
	}
	return ResolveCitadelServerByEndpoint(c.connection.state.endpoint)
}

// SwitchServer transitions directly to the next server through a fresh SquadBrowser
// join URL without terminating the running Squad process.
func (c *Controller) SwitchServer(ctx context.Context, joinURL string) error {
	return c.switchResolved(ctx, func(context.Context) (string, error) { return joinURL, nil })
}

func (c *Controller) switchResolved(ctx context.Context, resolve func(context.Context) (string, error)) error {
	logger.Info("SwitchServer initiated without restarting Squad")
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		logger.Warn("Server switch cancelled by context", "error", err)
		return err
	}
	return c.connectResolved(ctx, resolve)
}

// CloseSquad terminates the running game via the platform adapter.
func (c *Controller) CloseSquad() error {
	logger.Info("Requesting platform adapter to terminate Squad process")
	err := c.platform.CloseGame()
	if err != nil {
		logger.Error("Platform adapter failed to terminate Squad process", "error", err)
	}
	return err
}

// CheckSquadRunning distinguishes a stopped game from a failed process inspection.
func (c *Controller) CheckSquadRunning() (bool, error) { return c.platform.CheckSquadRunning() }

// WaitForSquad waits for the process without owning an unbounded goroutine.
func (c *Controller) WaitForSquad(ctx context.Context, pollInterval time.Duration) error {
	isRunning, err := c.CheckSquadRunning()
	if err != nil {
		return err
	}
	if isRunning {
		return nil
	}
	if pollInterval <= 0 {
		pollInterval = time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			isRunning, err := c.CheckSquadRunning()
			if err != nil {
				return err
			}
			if isRunning {
				return nil
			}
		}
	}
}

// WaitForSquadExit waits until the platform adapter no longer sees Squad.
func (c *Controller) WaitForSquadExit(ctx context.Context, pollInterval time.Duration) error {
	isRunning, err := c.CheckSquadRunning()
	if err != nil {
		return err
	}
	if !isRunning {
		return nil
	}
	if pollInterval <= 0 {
		pollInterval = defaultProcessPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			isRunning, err := c.CheckSquadRunning()
			if err != nil {
				return err
			}
			if !isRunning {
				return nil
			}
		}
	}
}
