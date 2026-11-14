package seed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/config"
	"citadel-seeder/internal/logger"
	"citadel-seeder/internal/platform"
)

const (
	progressTickInterval            = time.Second
	statusPollInterval              = 15 * time.Second
	minimumActiveSeedForPowerAction = 3 * time.Minute
	defaultPowerActionCountdown     = 30 // seconds
	gameExitBeforePowerTimeout      = 30 * time.Second
	defaultConnectionTimeout        = 12 * time.Minute
)

var ErrSeedStopped = errors.New("seed session stopped")

// State is the public seed-engine state exposed through the Wails bridge.
type State string

const (
	StateIdle          State = "idle"
	StatePreparing     State = "preparing"
	StateTransitioning State = "transitioning"
	StateConnecting    State = "connecting"
	StateActive        State = "active"
	StateCompleted     State = "completed"
	StateTimedOut      State = "timed_out"
	StateStopped       State = "stopped"
	StateFailed        State = "failed"
)

func (s State) IsActive() bool {
	return s == StatePreparing || s == StateTransitioning || s == StateConnecting || s == StateActive
}

const (
	PostSeedDoNothing  = "do_nothing"
	PostSeedCloseSquad = "close_squad"
	PostSeedSleep      = "sleep"
	PostSeedShutdown   = "shutdown"
)

const (
	TimeoutAbort        = "abort"
	TimeoutRetry        = "retry"
	TimeoutNotifyOnly   = "notify_only"
	TimeoutContinueNext = "continue_next"
	TimeoutCloseSquad   = "close_squad"
	TimeoutSleep        = "sleep"
	TimeoutShutdown     = "shutdown"
)

type Plan struct {
	ServerID           string   `json:"serverId"`
	ServerIDs          []string `json:"serverIds,omitempty"`
	ServerName         string   `json:"serverName"`
	ServerIP           string   `json:"serverIp,omitempty"`
	GamePort           int      `json:"gamePort,omitempty"`
	QueryPort          int      `json:"queryPort,omitempty"`
	TargetPlayerCount  int      `json:"targetPlayerCount"`
	MaxDurationMinutes int      `json:"maxDurationMinutes"`
	ServerLimit        int      `json:"serverLimit"`
	PostSeedAction     string   `json:"postSeedAction"`
	TimeoutAction      string   `json:"timeoutAction"`
	AutoRecoverCrash     bool     `json:"autoRecoverCrash,omitempty"`
	ForceRunningSettings bool     `json:"forceRunningSettings,omitempty"`
}

type Session struct {
	Revision           uint64          `json:"revision"`
	State              State           `json:"state"`
	HasObservedGame    bool            `json:"hasObservedGame"`
	Server             *citadel.Server `json:"server,omitempty"`
	Plan               *Plan           `json:"plan,omitempty"`
	StartedAt          *time.Time      `json:"startedAt,omitempty"`
	CompletedAt        *time.Time      `json:"completedAt,omitempty"`
	InitialPlayerCount int             `json:"initialPlayerCount"`
	CurrentPlayerCount int             `json:"currentPlayerCount"`
	TargetPlayerCount  int             `json:"targetPlayerCount"`
	QueuePosition      int             `json:"queuePosition"`
	ElapsedSeconds     int             `json:"elapsedSeconds"`
	RemainingSeconds   *int            `json:"remainingSeconds"`
	CurrentQueueIndex  int             `json:"currentQueueIndex"`
	TotalQueueCount    int             `json:"totalQueueCount"`
	SkippedServerCount int             `json:"skippedServerCount"`
	ErrorMessage       string          `json:"errorMessage,omitempty"`
}

type ServerProvider interface {
	GetServers(ctx context.Context) ([]citadel.Server, error)
	ResolveJoinURL(ctx context.Context, serverID string) (string, error)
}

type GameController interface {
	ConnectResolved(ctx context.Context, resolve func(context.Context) (string, error), shouldSwitch bool) error
	Connect(ctx context.Context, joinURL string) error
	SwitchServer(ctx context.Context, joinURL string) error
	CloseSquad() error
	CheckSquadRunning() (bool, error)
	CheckServerConnection(server citadel.Server) (bool, error)
	GetObservedConnectedServerID() string
	WaitForSquadExit(ctx context.Context, pollInterval time.Duration) error
}

type SystemController interface {
	PreventSleep(ctx context.Context, reason string) (platform.ReleaseFunc, error)
	ExecutePowerAction(action string) error
	CancelPowerAction() error
}

type SettingsProvider interface {
	Get() config.AppSettings
}

type StateChangeCallback func(session Session)

type Engine struct {
	mu                sync.RWMutex
	session           Session
	cancelFn          context.CancelFunc
	done              chan struct{}
	runID             uint64
	revision          uint64
	system            SystemController
	game              GameController
	servers           ServerProvider
	settings          SettingsProvider
	onStateChanged    StateChangeCallback
	progressEvery     time.Duration
	statusEvery       time.Duration
	durationUnit      time.Duration
	connectionTimeout time.Duration
	skipServerCh      chan struct{}
	connectionHook    func(connected bool)

	// Power action coordination & countdown
	powerCountdownCb       func(action string, secondsLeft int)
	cancelPowerMu          sync.Mutex
	cancelPowerCh          chan struct{}
	powerPending           bool
	isPowerActionScheduled bool
}

func NewEngine(
	system SystemController,
	game GameController,
	servers ServerProvider,
	settings SettingsProvider,
	onStateChanged StateChangeCallback,
) *Engine {
	return &Engine{
		session:           Session{State: StateIdle},
		system:            system,
		game:              game,
		servers:           servers,
		settings:          settings,
		onStateChanged:    onStateChanged,
		progressEvery:     progressTickInterval,
		statusEvery:       statusPollInterval,
		durationUnit:      time.Minute,
		connectionTimeout: defaultConnectionTimeout,
	}
}

// SetPowerCountdownCallback registers a listener for impending power actions (countdown from 30 to 0).
func (e *Engine) SetPowerCountdownCallback(cb func(action string, secondsLeft int)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.powerCountdownCb = cb
}

// SetConnectionHook registers a callback that is notified whenever Squad connects or disconnects in-game.
func (e *Engine) SetConnectionHook(hook func(connected bool)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.connectionHook = hook
}

func (e *Engine) notifyConnectionState(connected bool) {
	e.mu.RLock()
	hook := e.connectionHook
	e.mu.RUnlock()
	if hook != nil {
		hook(connected)
	}
}

// CancelPowerAction aborts any pending power action countdown and calls the system adapter cancellation.
func (e *Engine) CancelPowerAction() error {
	e.cancelPowerMu.Lock()
	defer e.cancelPowerMu.Unlock()
	if e.cancelPowerCh != nil {
		select {
		case <-e.cancelPowerCh:
		default:
			close(e.cancelPowerCh)
		}
		e.cancelPowerCh = nil
	}
	e.powerPending = false
	if e.isPowerActionScheduled && e.system != nil {
		if err := e.system.CancelPowerAction(); err != nil {
			return fmt.Errorf("cancel Sentry's scheduled power action: %w", err)
		}
		e.isPowerActionScheduled = false
	}
	logger.Info("Impending power action cancelled by user or application request")
	return nil
}

func (e *Engine) getMinPowerActionDuration() time.Duration {
	if e.durationUnit > 0 && e.durationUnit < time.Minute {
		return 10 * time.Millisecond
	}
	return minimumActiveSeedForPowerAction
}

func (e *Engine) getPowerCountdownSeconds() int {
	if e.durationUnit > 0 && e.durationUnit < time.Minute {
		return 0
	}
	return defaultPowerActionCountdown
}

// BuildQueue creates a deterministic queue from the authoritative visible fleet.
func BuildQueue(plan Plan, fleet []citadel.Server) ([]citadel.Server, error) {
	if len(fleet) == 0 {
		return nil, errors.New("no Citadel servers are currently available")
	}

	byID := make(map[string]citadel.Server, len(fleet))
	for _, server := range fleet {
		byID[server.ID] = server
	}

	var ordered []citadel.Server
	if len(plan.ServerIDs) > 0 {
		seen := make(map[string]struct{}, len(plan.ServerIDs))
		for _, serverID := range plan.ServerIDs {
			if _, ok := seen[serverID]; ok {
				continue
			}
			server, ok := byID[serverID]
			if !ok {
				return nil, fmt.Errorf("server not found: %s", serverID)
			}
			if !isSeedableStatus(server.Status) {
				return nil, fmt.Errorf("server is unavailable for seeding: %s (status %s)", server.Name, server.Status)
			}
			seen[serverID] = struct{}{}
			ordered = append(ordered, server)
		}
	} else {
		startIndex := -1
		for index := range fleet {
			if fleet[index].ID == plan.ServerID {
				startIndex = index
				break
			}
		}
		if startIndex < 0 {
			return nil, fmt.Errorf("server not found: %s", plan.ServerID)
		}
		if !isSeedableStatus(fleet[startIndex].Status) {
			return nil, fmt.Errorf("server is unavailable for seeding: %s (status %s)", fleet[startIndex].Name, fleet[startIndex].Status)
		}

		ordered = append(ordered, fleet[startIndex])
		for offset := 1; offset < len(fleet); offset++ {
			server := fleet[(startIndex+offset)%len(fleet)]
			if !isSeedableStatus(server.Status) {
				continue
			}
			ordered = append(ordered, server)
		}
	}

	if len(ordered) == 0 {
		return nil, errors.New("no seedable Citadel servers are available")
	}
	if plan.ServerLimit > 0 && plan.ServerLimit < len(ordered) {
		ordered = ordered[:plan.ServerLimit]
	}
	return ordered, nil
}

func isSeedableStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "seeding", "full":
		return true
	default:
		return false
	}
}

// GetSession returns a race-safe snapshot of the current session.
func (e *Engine) GetSession() Session {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneSession(e.session)
}

// HasRunningWorker reports whether a queue worker can still launch or switch Squad.
func (e *Engine) HasRunningWorker() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.done != nil
}

// CurrentRunID returns the identity of the latest seed worker.
func (e *Engine) CurrentRunID() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.runID
}

// IsRunningRun reports whether an identified worker still owns active seeding.
func (e *Engine) IsRunningRun(runID uint64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.runID == runID && e.done != nil && e.session.State.IsActive()
}

// Start begins one owned, cancellable queue run.
func (e *Engine) Start(parent context.Context, plan Plan, queue []citadel.Server) error {
	plan = normalizePlan(plan)
	if err := validatePlan(plan, queue); err != nil {
		return err
	}
	if parent == nil {
		parent = context.Background()
	}

	queueCopy := append([]citadel.Server(nil), queue...)
	runCtx, cancel := context.WithCancel(parent)
	remaining := int(e.planDuration(plan).Seconds())
	firstServer := queueCopy[0]

	e.mu.Lock()
	if e.done != nil {
		e.mu.Unlock()
		cancel()
		return errors.New("stop the previous seed worker and wait for it to finish before starting another")
	}

	e.runID++
	e.revision++
	runID := e.runID
	done := make(chan struct{})
	e.cancelFn = cancel
	e.done = done
	e.skipServerCh = make(chan struct{}, 1)
	e.session = Session{
		Revision:           e.revision,
		State:              StatePreparing,
		Server:             &firstServer,
		Plan:               &plan,
		InitialPlayerCount: firstServer.CurrentPlayers,
		CurrentPlayerCount: firstServer.CurrentPlayers,
		TargetPlayerCount:  plan.TargetPlayerCount,
		QueuePosition:      firstServer.PublicQueue,
		RemainingSeconds:   &remaining,
		CurrentQueueIndex:  1,
		TotalQueueCount:    len(queueCopy),
	}
	snapshot := cloneSession(e.session)
	e.mu.Unlock()
	e.publish(snapshot)

	go e.run(runCtx, runID, plan, queueCopy, done)
	return nil
}

// Stop cancels the active run and immediately publishes the stopped state.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if e.done == nil {
		e.mu.Unlock()
		return nil
	}

	cancel := e.cancelFn
	if e.session.State == StateFailed {
		e.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return nil
	}
	now := time.Now()
	e.session.State = StateStopped
	e.session.CompletedAt = &now
	e.session.ErrorMessage = ""
	e.revision++
	e.session.Revision = e.revision
	snapshot := cloneSession(e.session)
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	e.publish(snapshot)
	return nil
}

// SkipCurrentServer advances the seed queue to the next server if a mission is running.
func (e *Engine) SkipCurrentServer() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.session.State.IsActive() {
		return errors.New("no active seeding mission to skip")
	}
	if e.skipServerCh != nil {
		select {
		case e.skipServerCh <- struct{}{}:
			logger.Info("Skip requested for current server")
		default:
		}
	}
	return nil
}


// WaitForStop waits for worker cleanup without launching or cancelling another run.
func (e *Engine) WaitForStop(ctx context.Context) error {
	e.mu.RLock()
	done := e.done
	e.mu.RUnlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PerformRunOperation serializes a native operation with worker state changes.
// The operation must not call back into the engine.
func (e *Engine) PerformRunOperation(runID uint64, operation func() error) (bool, error) {
	e.mu.Lock()
	if e.runID != runID || e.done == nil || !e.session.State.IsActive() {
		e.mu.Unlock()
		return false, nil
	}
	err := operation()
	var snapshot *Session
	var cancel context.CancelFunc
	if err != nil {
		now := time.Now()
		e.session.State = StateFailed
		e.session.CompletedAt = &now
		e.session.ErrorMessage = err.Error()
		e.revision++
		e.session.Revision = e.revision
		updated := cloneSession(e.session)
		snapshot = &updated
		cancel = e.cancelFn
	}
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if snapshot != nil {
		e.publish(*snapshot)
	}
	return true, err
}

// AbortRun fails and cancels only the identified active worker.
func (e *Engine) AbortRun(runID uint64, failure error) bool {
	if failure == nil {
		return false
	}
	e.mu.Lock()
	if e.runID != runID || e.done == nil || !e.session.State.IsActive() {
		e.mu.Unlock()
		return false
	}
	cancel := e.cancelFn
	now := time.Now()
	e.session.State = StateFailed
	e.session.CompletedAt = &now
	e.session.ErrorMessage = failure.Error()
	e.revision++
	e.session.Revision = e.revision
	snapshot := cloneSession(e.session)
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.publish(snapshot)
	return true
}

type serverOutcome int

type serverActivity struct {
	hasObservedGame bool
	duration        time.Duration
}

const (
	outcomeTargetReached serverOutcome = iota
	outcomeTimedOut
	outcomeSkipped
)

func (e *Engine) run(ctx context.Context, runID uint64, plan Plan, queue []citadel.Server, done chan struct{}) {
	logger.Info("Seed engine worker loop started", "runID", runID, "queueLength", len(queue))
	defer e.endRun(runID, done)

	var activeSeedingOccurred bool
	var observedGameDuration time.Duration
	var timedOutServers int

	// Pre-flight safety check: Are ALL servers in the queue already meeting target threshold?
	preflightAllTargetReached := true
	for _, srv := range queue {
		if srv.CurrentPlayers < plan.TargetPlayerCount {
			preflightAllTargetReached = false
			break
		}
	}
	if preflightAllTargetReached {
		logger.Warn("Pre-flight safety notice: all servers in seed queue already meet or exceed target player count before launch",
			"runID", runID,
			"target", plan.TargetPlayerCount,
		)
	}

	var releaseSleep platform.ReleaseFunc
	release := func() {
		if releaseSleep == nil {
			return
		}
		logger.Info("Releasing system sleep lock", "runID", runID)
		releaseSleep()
		releaseSleep = nil
	}
	defer release()

	if e.settings.Get().Power.PreventSleepDuringSeed {
		logger.Info("Inhibiting system sleep for active seed mission", "runID", runID)
		var err error
		releaseSleep, err = e.system.PreventSleep(ctx, "Citadel Sentry seed session")
		if err != nil {
			logger.Error("Failed to inhibit system sleep", "runID", runID, "error", err)
			e.fail(runID, fmt.Errorf("prevent system sleep: %w", err))
			return
		}
	}

	isRetry := false
	for queueIndex := 0; queueIndex < len(queue); {
		srv := queue[queueIndex]
		logger.Info("Executing seed queue step", "runID", runID, "step", queueIndex+1, "total", len(queue), "server", srv.Name)
		outcome, activity, err := e.runServer(ctx, runID, plan, srv, queueIndex, len(queue), queueIndex > 0 || isRetry)
		observedGameDuration += activity.duration
		if activity.hasObservedGame {
			activeSeedingOccurred = true
		}
		if err != nil {
			if errors.Is(err, ErrSeedStopped) || errors.Is(err, context.Canceled) {
				logger.Info("Seed run cancelled or stopped", "runID", runID, "server", srv.Name)
				e.markStopped(runID)
				return
			}
			logger.Error("Seed step encountered error", "runID", runID, "server", srv.Name, "error", err)
			e.fail(runID, err)
			return
		}

		switch outcome {
		case outcomeTargetReached:
			isRetry = false
			logger.Info("Target player count reached for server", "runID", runID, "server", srv.Name, "target", plan.TargetPlayerCount)
			queueIndex++
			if queueIndex < len(queue) {
				logger.Info("Advancing to next server in queue", "runID", runID, "nextServer", queue[queueIndex].Name)
				continue
			}
			if timedOutServers > 0 {
				logger.Info("Seed queue finished with servers skipped after timeout", "runID", runID)
				e.finish(runID, StateTimedOut)
				return
			}
			logger.Info("All servers in seed queue completed successfully", "runID", runID)
			e.finish(runID, StateCompleted)
			release()
			if err := e.performPostSeedAction(ctx, plan.PostSeedAction, activeSeedingOccurred, observedGameDuration, preflightAllTargetReached); err != nil {
				e.fail(runID, err)
			}
			return
		case outcomeSkipped:
			isRetry = false
			e.update(runID, func(session *Session) { session.SkippedServerCount++ })
			queueIndex++
			if queueIndex < len(queue) {
				logger.Info("Advancing to next server after user skip", "runID", runID, "nextServer", queue[queueIndex].Name)
				continue
			}
			logger.Info("Seed queue finished after user skip of last server", "runID", runID)
			e.finish(runID, StateCompleted)
			release()
			return
		case outcomeTimedOut:
			logger.Warn("Seed timeout reached for server", "runID", runID, "server", srv.Name, "action", plan.TimeoutAction)
			switch plan.TimeoutAction {
			case TimeoutRetry:
				isRetry = true
				logger.Info("Retrying server after timeout", "runID", runID, "server", srv.Name)
				continue
			case TimeoutContinueNext:
				isRetry = false
				timedOutServers++
				e.update(runID, func(session *Session) { session.SkippedServerCount++ })
				queueIndex++
				if queueIndex < len(queue) {
					logger.Info("Advancing to next server after timeout", "runID", runID, "nextServer", queue[queueIndex].Name)
					continue
				}
			}
			e.finish(runID, StateTimedOut)
			release()
			if plan.TimeoutAction == TimeoutCloseSquad || plan.TimeoutAction == TimeoutSleep || plan.TimeoutAction == TimeoutShutdown {
				if err := e.performPostSeedAction(ctx, plan.TimeoutAction, activeSeedingOccurred, observedGameDuration, false); err != nil {
					e.fail(runID, err)
				}
			}
			return
		}
	}
}

func (e *Engine) runServer(
	ctx context.Context,
	runID uint64,
	plan Plan,
	server citadel.Server,
	queueIndex int,
	queueCount int,
	shouldSwitch bool,
) (outcome serverOutcome, activity serverActivity, failure error) {
	duration := e.planDuration(plan)
	remaining := int(duration.Seconds())

	logger.Info("Starting server seeding run",
		"runID", runID,
		"server", server.Name,
		"serverID", server.ID,
		"currentPlayers", server.CurrentPlayers,
		"targetPlayers", plan.TargetPlayerCount,
		"maxDuration", duration.String(),
	)

	if !e.update(runID, func(session *Session) {
		serverCopy := server
		planCopy := plan
		session.State = StatePreparing
		session.Server = &serverCopy
		session.Plan = &planCopy
		session.StartedAt = nil
		session.CompletedAt = nil
		session.InitialPlayerCount = server.CurrentPlayers
		session.CurrentPlayerCount = server.CurrentPlayers
		session.TargetPlayerCount = plan.TargetPlayerCount
		session.QueuePosition = server.PublicQueue
		session.ElapsedSeconds = 0
		session.RemainingSeconds = &remaining
		session.CurrentQueueIndex = queueIndex + 1
		session.TotalQueueCount = queueCount
		session.ErrorMessage = ""
	}) {
		return outcomeTimedOut, activity, ErrSeedStopped
	}

	current, err := e.fetchServer(ctx, server.ID)
	if err != nil {
		return outcomeTimedOut, activity, fmt.Errorf("verify live server before launching Squad: %w", err)
	}
	if plan.TargetPlayerCount > current.MaxPlayers {
		return outcomeTimedOut, activity, fmt.Errorf("target player count exceeds live capacity of %s", current.Name)
	}
	server = current
	if !e.update(runID, func(session *Session) {
		serverCopy := server
		session.Server = &serverCopy
		session.InitialPlayerCount = server.CurrentPlayers
		session.CurrentPlayerCount = server.CurrentPlayers
		session.QueuePosition = server.PublicQueue
	}) {
		return outcomeTimedOut, activity, ErrSeedStopped
	}
	if server.CurrentPlayers >= plan.TargetPlayerCount {
		logger.Info("Server already meets or exceeds target player threshold", "server", server.Name, "current", server.CurrentPlayers, "target", plan.TargetPlayerCount)
		return outcomeTargetReached, activity, nil
	}

	if !e.update(runID, func(session *Session) {
		if shouldSwitch {
			session.State = StateTransitioning
			return
		}
		session.State = StateConnecting
	}) {
		return outcomeTimedOut, activity, ErrSeedStopped
	}

	if err := ctx.Err(); err != nil {
		return outcomeTimedOut, activity, ErrSeedStopped
	}
	resolve := func(ctx context.Context) (string, error) { return e.servers.ResolveJoinURL(ctx, server.ID) }
	if err := e.game.ConnectResolved(ctx, resolve, shouldSwitch); err != nil {
		logger.Error("Failed to launch Squad for server", "server", server.Name, "error", err)
		return outcomeTimedOut, activity, fmt.Errorf("launch Squad for %s: %w", server.Name, err)
	}

	return e.monitorServer(ctx, runID, plan, server)
}

func (e *Engine) planDuration(plan Plan) time.Duration {
	unit := e.durationUnit
	if unit <= 0 {
		unit = time.Minute
	}
	return time.Duration(plan.MaxDurationMinutes) * unit
}

func (e *Engine) fetchServer(ctx context.Context, serverID string) (citadel.Server, error) {
	servers, err := e.servers.GetServers(ctx)
	if err != nil {
		return citadel.Server{}, err
	}
	for _, server := range servers {
		if server.ID == serverID {
			if !isSeedableStatus(server.Status) {
				return citadel.Server{}, fmt.Errorf("server %s is not ready for seeding (status %s)", serverID, server.Status)
			}
			if server.MaxPlayers <= 0 {
				return citadel.Server{}, fmt.Errorf("live capacity of server %s is unknown", serverID)
			}
			return server, nil
		}
	}
	return citadel.Server{}, fmt.Errorf("server disappeared from public fleet: %s", serverID)
}

func (e *Engine) updateServerTelemetry(runID uint64, server citadel.Server) {
	e.update(runID, func(session *Session) {
		serverCopy := server
		session.Server = &serverCopy
		session.CurrentPlayerCount = server.CurrentPlayers
		session.QueuePosition = server.PublicQueue
	})
}

func (e *Engine) performPostSeedAction(
	ctx context.Context,
	action string,
	activeSeedingOccurred bool,
	observedGameDuration time.Duration,
	preflightFull bool,
) error {
	if ctx.Err() != nil {
		return nil
	}
	logger.Info("Evaluating post-seed action",
		"action", action,
		"activeSeeding", activeSeedingOccurred,
		"observedGameDuration", observedGameDuration.String(),
		"preflightFull", preflightFull,
	)

	switch action {
	case PostSeedDoNothing:
		return nil

	case PostSeedCloseSquad:
		if err := e.game.CloseSquad(); err != nil {
			logger.Error("Failed to close Squad after seeding", "error", err)
			return fmt.Errorf("close Squad after seeding: %w", err)
		}
		return nil

	case PostSeedSleep, PostSeedShutdown:
		// Safety Guard 1: If preflight showed all servers already full before start, DO NOT power off
		if preflightFull {
			logger.Warn("All servers already met player target before seeding started. Power action aborted for workstation safety.",
				"action", action,
			)
			return nil
		}

		// Safety Guard 2: If Squad was never actually launched/active, DO NOT power off
		if !activeSeedingOccurred {
			logger.Warn("No active Squad session was run during seeding. Power action aborted for workstation safety.",
				"action", action,
			)
			return nil
		}

		// Safety Guard 3: Minimum active seeding duration check (e.g. 3 minutes)
		minDuration := e.getMinPowerActionDuration()
		if observedGameDuration < minDuration {
			logger.Warn("Observed Squad activity ended before the minimum power-action duration",
				"observedGameDuration", observedGameDuration.String(),
				"minRequired", minDuration.String(),
				"action", action,
			)
			return nil
		}

		// Step 1: Cleanly terminate Squad game process first
		isRunning, err := e.game.CheckSquadRunning()
		if err != nil {
			return fmt.Errorf("verify game before power action: %w", err)
		}
		if isRunning {
			logger.Info("Terminating Squad before executing system power action", "action", action)
			if err := e.game.CloseSquad(); err != nil {
				return fmt.Errorf("close Squad before power action: %w", err)
			}
			waitCtx, cancel := context.WithTimeout(ctx, gameExitBeforePowerTimeout)
			waitErr := e.game.WaitForSquadExit(waitCtx, progressTickInterval)
			cancel()
			if waitErr != nil {
				return fmt.Errorf("verify Squad exited before power action: %w", waitErr)
			}
		}

		// Step 2: Initialize cancellation channel & countdown
		e.cancelPowerMu.Lock()
		cancelCh := make(chan struct{})
		e.cancelPowerCh = cancelCh
		e.powerPending = true
		e.cancelPowerMu.Unlock()

		countdownSeconds := e.getPowerCountdownSeconds()
		logger.Info("Starting cancelable countdown for power action",
			"action", action,
			"countdownSeconds", countdownSeconds,
		)

		for remaining := countdownSeconds; remaining > 0; remaining-- {
			e.mu.RLock()
			cb := e.powerCountdownCb
			e.mu.RUnlock()
			if cb != nil {
				cb(action, remaining)
			}

			select {
			case <-ctx.Done():
				logger.Info("Power action aborted: engine context cancelled")
				_ = e.CancelPowerAction()
				return nil
			case <-cancelCh:
				logger.Info("Power action aborted: cancelled by user")
				return nil
			case <-time.After(time.Second):
			}
		}

		// Notify 0 seconds remaining
		e.mu.RLock()
		cb := e.powerCountdownCb
		e.mu.RUnlock()
		if cb != nil {
			cb(action, 0)
		}

		// Step 3: Execute the system power action
		e.cancelPowerMu.Lock()
		defer e.cancelPowerMu.Unlock()
		select {
		case <-ctx.Done():
			e.powerPending = false
			e.cancelPowerCh = nil
			return nil
		case <-cancelCh:
			e.powerPending = false
			e.cancelPowerCh = nil
			return nil
		default:
		}
		e.powerPending = false
		e.cancelPowerCh = nil

		logger.Info("Executing system power action now", "action", action)
		if err := e.system.ExecutePowerAction(action); err != nil {
			logger.Error("Failed to execute power action", "action", action, "error", err)
			return fmt.Errorf("execute power action %s: %w", action, err)
		}
		e.isPowerActionScheduled = action == PostSeedShutdown
		return nil

	default:
		logger.Warn("Unsupported post-seed action requested", "action", action)
		return fmt.Errorf("unsupported post-seed action: %s", action)
	}
}

func (e *Engine) finish(runID uint64, state State) {
	logger.Info("Seed session reached terminal state", "runID", runID, "state", state)
	now := time.Now()
	zero := 0
	e.update(runID, func(session *Session) {
		session.State = state
		session.CompletedAt = &now
		session.RemainingSeconds = &zero
		session.ErrorMessage = ""
	})
}

func (e *Engine) fail(runID uint64, err error) {
	logger.Error("Seed session failed with error", "runID", runID, "error", err)
	now := time.Now()
	e.update(runID, func(session *Session) {
		session.State = StateFailed
		session.CompletedAt = &now
		session.ErrorMessage = err.Error()
	})
}

func (e *Engine) markStopped(runID uint64) {
	logger.Info("Seed session marked stopped", "runID", runID)
	now := time.Now()
	e.update(runID, func(session *Session) {
		if session.State == StateStopped {
			return
		}
		session.State = StateStopped
		session.CompletedAt = &now
		session.ErrorMessage = ""
	})
}

func (e *Engine) update(runID uint64, mutate func(session *Session)) bool {
	e.mu.Lock()
	if e.runID != runID || e.session.State == StateStopped || e.session.State == StateFailed {
		e.mu.Unlock()
		return false
	}
	mutate(&e.session)
	e.revision++
	e.session.Revision = e.revision
	snapshot := cloneSession(e.session)
	e.mu.Unlock()
	e.publish(snapshot)
	return true
}

func (e *Engine) endRun(runID uint64, done chan struct{}) {
	e.mu.Lock()
	if e.runID == runID {
		e.cancelFn = nil
		e.done = nil
	}
	e.mu.Unlock()
	close(done)
}

func (e *Engine) publish(session Session) {
	if e.onStateChanged != nil {
		e.onStateChanged(session)
	}
}

func cloneSession(session Session) Session {
	cloned := session
	if session.Server != nil {
		server := *session.Server
		cloned.Server = &server
	}
	if session.Plan != nil {
		plan := *session.Plan
		plan.ServerIDs = append([]string(nil), session.Plan.ServerIDs...)
		cloned.Plan = &plan
	}
	if session.StartedAt != nil {
		startedAt := *session.StartedAt
		cloned.StartedAt = &startedAt
	}
	if session.CompletedAt != nil {
		completedAt := *session.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	if session.RemainingSeconds != nil {
		remaining := *session.RemainingSeconds
		cloned.RemainingSeconds = &remaining
	}
	return cloned
}

func normalizePlan(plan Plan) Plan {
	switch plan.PostSeedAction {
	case "", "stay":
		plan.PostSeedAction = PostSeedDoNothing
	case "close_game", "exit_app":
		plan.PostSeedAction = PostSeedCloseSquad
	}
	switch plan.TimeoutAction {
	case "":
		plan.TimeoutAction = TimeoutAbort
	case "close_game", "exit_app":
		plan.TimeoutAction = TimeoutCloseSquad
	}
	return plan
}

func validatePlan(plan Plan, queue []citadel.Server) error {
	if plan.ServerID == "" && len(plan.ServerIDs) == 0 {
		return errors.New("seed plan requires at least one server")
	}
	if plan.TargetPlayerCount <= 0 {
		return errors.New("target player count must be greater than zero")
	}
	if plan.MaxDurationMinutes <= 0 || plan.MaxDurationMinutes > config.MaxSeedDurationMinutes {
		return errors.New("maximum seed duration must fit a positive duration")
	}
	if plan.ServerLimit < 0 {
		return errors.New("server limit cannot be negative")
	}
	if len(queue) == 0 {
		return errors.New("seed queue cannot be empty")
	}
	for _, server := range queue {
		if server.MaxPlayers <= 0 {
			return fmt.Errorf("capacity of %s is unknown", server.Name)
		}
		if plan.TargetPlayerCount > server.MaxPlayers {
			return fmt.Errorf("target player count exceeds capacity of %s", server.Name)
		}
	}

	switch plan.PostSeedAction {
	case PostSeedDoNothing, PostSeedCloseSquad, PostSeedSleep, PostSeedShutdown:
	default:
		return fmt.Errorf("unsupported post-seed action: %s", plan.PostSeedAction)
	}
	switch plan.TimeoutAction {
	case TimeoutAbort, TimeoutRetry, TimeoutNotifyOnly, TimeoutContinueNext, TimeoutCloseSquad, TimeoutSleep, TimeoutShutdown:
	default:
		return fmt.Errorf("unsupported timeout action: %s", plan.TimeoutAction)
	}
	return nil
}
