package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/config"
	"citadel-seeder/internal/game"
	"citadel-seeder/internal/logger"
	"citadel-seeder/internal/platform"
	"citadel-seeder/internal/seed"
	"citadel-seeder/internal/telemetry"
	"citadel-seeder/internal/updater"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	appShutdownWaitTimeout = 5 * time.Second
	priorityPollInterval   = time.Second
	priorityStartupTimeout = 90 * time.Second
)

// App struct
type App struct {
	ctx               context.Context
	cfgMgr            *config.Manager
	platform          platform.Adapter
	gameCtrl          *game.Controller
	citadel           *citadel.Client
	seeder            *seed.Engine
	squadConfig       *game.ConfigManager
	isPreparingPotato atomic.Bool
	updateInProgress  atomic.Bool
	updater           *updater.Manager
	telemetryReporter *telemetry.Reporter
	operationMu       sync.Mutex
	directJoinCancel  context.CancelFunc
	directJoinID      uint64
	potatoRunID       uint64
	backgroundCancel    context.CancelFunc
	isShuttingDown      atomic.Bool
	shutdownOnce        sync.Once
	lastDispatchedRunID atomic.Uint64
	telemetryWg         sync.WaitGroup
}

// NewApp creates a new App application struct
func NewApp() (*App, error) {
	plt := platform.NewAdapter()
	priorityRecoveryError := plt.RestoreProcessPriority()
	if priorityRecoveryError != nil {
		logger.Error("Pending process priority restoration failed before settings initialization", "error", priorityRecoveryError)
	}
	sc, err := game.NewConfigManager()
	if err != nil {
		return nil, fmt.Errorf("initialize Squad settings recovery: %w", err)
	}
	configRecoveryError := restoreInterruptedSquadConfig(plt, sc)
	if configRecoveryError != nil {
		logger.Error("Interrupted Squad configuration recovery failed before settings initialization", "error", configRecoveryError)
	}
	_ = sc.EnsureInitialBaselineCaptured()
	initialSettings := config.DefaultSettings()
	initialSettings.Potato.LowCpuPriority = plt.CheckProcessPrioritySupport() == nil
	cfgMgr, err := config.NewManagerWithDefaults(initialSettings)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("initialize settings: %w", err), priorityRecoveryError, configRecoveryError)
	}
	gc := game.NewController(plt)
	gc.SetJoinJitterMax(3500 * time.Millisecond)
	cc := citadel.NewClient(cfgMgr.Get().WebApiBaseUrl)
	gc.SetPlayerVerifier(cc.HasServerPlayer)

	app := &App{
		cfgMgr:      cfgMgr,
		platform:    plt,
		gameCtrl:    gc,
		citadel:     cc,
		squadConfig: sc,
		updater:     updater.NewManager(applicationVersion()),
	}

	gpu := plt.DetectGPU()
	reporter := telemetry.NewReporter(
		initialSettings.General.TelemetryOptIn,
		initialSettings.WebApiBaseUrl+"/api/telemetry/sentry",
		applicationVersion(),
	)
	reporter.SetHardware(telemetry.HardwareConfig{
		OSCaption:     plt.Name(),
		GPUModel:      gpu.Model,
		GPUVendor:     string(gpu.Vendor),
		SentryVersion: applicationVersion(),
	})
	app.telemetryReporter = reporter

	app.seeder = seed.NewEngine(plt, gc, cc, cfgMgr, func(session seed.Session) {
		if app.ctx != nil && !app.isShuttingDown.Load() {
			wruntime.EventsEmit(app.ctx, "citadel:seed:state_changed", session)
			wruntime.EventsEmit(app.ctx, "seed:session_update", session)
		}
		app.handleSessionStateChange(session)
	})

	app.seeder.SetPowerCountdownCallback(func(action string, secondsLeft int) {
		if app.ctx != nil && !app.isShuttingDown.Load() {
			wruntime.EventsEmit(app.ctx, "citadel:power_action_impending", map[string]any{
				"action":      action,
				"secondsLeft": secondsLeft,
			})
		}
	})

	app.seeder.SetConnectionHook(func(connected bool) {
		if app.squadConfig == nil || !app.squadConfig.HasActiveBackup() {
			return
		}
		potatoCfg := app.cfgMgr.Get().Potato
		if !potatoCfg.Enabled {
			return
		}
		if connected {
			logger.Info("In-game connection confirmed; clamping FPS to Potato Mode target", "targetFps", potatoCfg.FpsLimit)
			if err := app.squadConfig.UpdateActivePotatoFps(potatoCfg.FpsLimit); err != nil {
				logger.Warn("Unable to apply in-game FPS cap; preserving seeding session", "error", err)
				if app.ctx != nil && !app.isShuttingDown.Load() {
					wruntime.EventsEmit(app.ctx, "squad:potato_fps_error", map[string]any{
						"error":     err.Error(),
						"targetFps": potatoCfg.FpsLimit,
						"phase":     "connected",
					})
				}
			}
			if app.telemetryReporter != nil && app.telemetryReporter.IsOptIn() {
				app.telemetryReporter.RecordSample(telemetry.CollectDynamicSample(telemetry.CheckpointPostLaunch))
			}
		} else {
			logger.Info("In-game connection released; boosting to 60 FPS for fast loading", "fps", 60)
			if err := app.squadConfig.UpdateActivePotatoFps(60); err != nil {
				logger.Warn("Unable to restore loading FPS cap; preserving seeding session", "error", err)
				if app.ctx != nil && !app.isShuttingDown.Load() {
					wruntime.EventsEmit(app.ctx, "squad:potato_fps_error", map[string]any{
						"error":     err.Error(),
						"targetFps": 60,
						"phase":     "loading",
					})
				}
			}
		}
		if app.ctx != nil && !app.isShuttingDown.Load() {
			wruntime.EventsEmit(app.ctx, "squad:config_status", app.squadConfig.GetStatus())
		}
	})

	return app, nil
}

// restoreInterruptedSquadConfig recovers unfinished transactions without guessing app preferences.
func restoreInterruptedSquadConfig(adapter platform.Adapter, manager *game.ConfigManager) error {
	shouldRestore, err := manager.ShouldRestoreAutomatically(false)
	if err != nil || !shouldRestore {
		return err
	}
	isRunning, err := adapter.CheckSquadRunning()
	if err != nil {
		return fmt.Errorf("verify Squad before interrupted recovery: %w", err)
	}
	if isRunning {
		return nil
	}
	return manager.RestoreOriginalConfig()
}

// CheckForUpdates reads the latest stable Citadel Sentry GitHub release.
func (a *App) CheckForUpdates() (updater.Info, error) {
	return a.updater.Check(a.appContext())
}

// GetUpdateLastError reports a helper failure from the previous update attempt.
func (a *App) GetUpdateLastError() string {
	return updater.LastError()
}

// InstallUpdate verifies the selected release and restarts through a helper process.
func (a *App) InstallUpdate(version string) error {
	if a.ctx == nil {
		return fmt.Errorf("desktop runtime is not ready")
	}
	if !a.updateInProgress.CompareAndSwap(false, true) {
		return fmt.Errorf("an application update is already in progress")
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	updateStarted := false
	defer func() {
		if !updateStarted {
			a.updateInProgress.Store(false)
		}
	}()
	if a.isShuttingDown.Load() {
		return fmt.Errorf("Citadel Sentry is shutting down")
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() {
		return fmt.Errorf("stop seeding before updating Citadel Sentry")
	}
	if a.directJoinCancel != nil {
		return fmt.Errorf("stop the pending direct Squad join before updating Citadel Sentry")
	}
	if a.squadConfig != nil && a.squadConfig.HasActiveBackup() {
		return fmt.Errorf("finish pending Squad settings restoration before updating Citadel Sentry")
	}
	if err := a.updater.Start(a.ctx, version); err != nil {
		return err
	}
	updateStarted = true
	logger.Info("Verified application update downloaded; restarting", "version", version)
	go func(ctx context.Context) {
		time.Sleep(250 * time.Millisecond)
		wruntime.Quit(ctx)
	}(a.ctx)
	return nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.operationMu.Lock()
	if a.isShuttingDown.Load() {
		a.operationMu.Unlock()
		return
	}
	ctx, a.backgroundCancel = context.WithCancel(ctx)
	a.ctx = ctx
	a.operationMu.Unlock()

	// Restore a completed potato session only after Squad exits and stops writing its INI files.
	if err := a.restoreSquadAutomaticallyIfStopped(); err != nil {
		logger.Error("Could not recover Squad settings on startup", "error", err)
	}
	go a.watchSquadRestore(ctx)

	if err := a.squadConfig.SetShowEmptyServers(true); err != nil {
		logger.Error("Could not enable bShowEmptyServers on startup", "error", err)
	}

	if err := a.backupInitialSquadConfig(); err != nil {
		logger.Error("Could not create initial Squad settings backup", "error", err)
	}

	// Periodically emit telemetry ping
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status := a.GetSystemStatus()
				wruntime.EventsEmit(ctx, "bridge:status", status)
			}
		}
	}()
	if a.isShuttingDown.Load() {
		return
	}
	if err := updater.SignalReady(); err != nil {
		logger.Error("Could not confirm updated application startup", "error", err)
		return
	}
	go updater.CleanupPreviousHelper()
}

func (a *App) backupInitialSquadConfig() error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if a.squadConfig == nil || a.isShuttingDown.Load() {
		return nil
	}
	status := a.squadConfig.GetStatus()
	if !status.ConfigFound || status.HasBackup || status.IsPotatoMode {
		return nil
	}
	if err := a.requireSquadStopped(); err != nil {
		return err
	}
	if err := a.squadConfig.BackupConfig(); err != nil {
		return err
	}
	logger.Info("Created initial Squad settings backup", "configPath", status.ConfigPath)
	return nil
}

// shutdown releases owned background resources before the desktop process exits.
func (a *App) shutdown(_ context.Context) {
	a.shutdownOnce.Do(func() {
		a.isShuttingDown.Store(true)
		a.operationMu.Lock()
		if a.backgroundCancel != nil {
			a.backgroundCancel()
		}
		if a.directJoinCancel != nil {
			a.directJoinCancel()
		}
		if a.seeder != nil {
			if err := a.seeder.Stop(); err != nil {
				logger.Error("Could not stop seeding during shutdown", "error", err)
			}
			if err := a.seeder.CancelPowerAction(); err != nil {
				logger.Error("Could not cancel Sentry's power action during shutdown", "error", err)
			}
		}
		a.operationMu.Unlock()

		if a.seeder != nil {
			waitCtx, cancel := context.WithTimeout(context.Background(), appShutdownWaitTimeout)
			err := a.seeder.WaitForStop(waitCtx)
			cancel()
			if err != nil {
				logger.Error("Seed worker cleanup did not finish before shutdown", "error", err)
				return
			}
			if err := a.restoreSquadAutomaticallyIfStopped(); err != nil {
				logger.Error("Could not restore Squad settings during shutdown", "error", err)
			}
		}

		done := make(chan struct{})
		go func() {
			a.telemetryWg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			logger.Warn("Telemetry dispatch did not complete before shutdown timeout")
		}
	})
}

// GetSettings retrieves current application preferences
func (a *App) GetSettings() config.AppSettings {
	return a.cfgMgr.Get()
}

// SaveSettings persists updated application preferences
func (a *App) SaveSettings(settings config.AppSettings) bool {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return false
	}
	if err := a.cfgMgr.Save(settings); err != nil {
		logger.Error("Failed to save application settings", "error", err)
		return false
	}
	settings = a.cfgMgr.Get()
	a.citadel.SetBaseURL(settings.WebApiBaseUrl)
	if a.telemetryReporter != nil {
		a.telemetryReporter.SetOptIn(settings.General.TelemetryOptIn)
	}
	logger.Banner("APPLICATION SETTINGS UPDATED",
		fmt.Sprintf("WebApi BaseURL:        %s", settings.WebApiBaseUrl),
		fmt.Sprintf("General:               lang=%s, launchOnStartup=%t, minimizeToTray=%t, closeToTray=%t, notifications=%t",
			settings.General.Language, settings.General.LaunchOnStartup, settings.General.MinimizeToTray, settings.General.CloseToTray, settings.General.NotificationsEnabled),
		fmt.Sprintf("Potato Mode:           enabled=%t, fpsLimit=%d, lowRes=%t, lowCpu=%t, autoRestore=%t",
			settings.Potato.Enabled, settings.Potato.FpsLimit, settings.Potato.LowResolution, settings.Potato.LowCpuPriority, settings.Potato.AutoRestoreOnExit),
		fmt.Sprintf("Seeding Defaults:      target=%d, maxDuration=%dm, postAction=%s",
			settings.Seed.DefaultTargetPlayers, settings.Seed.DefaultMaxDurationMinutes, settings.Seed.DefaultPostSeedAction),
		fmt.Sprintf("Power Settings:        preventSleep=%t",
			settings.Power.PreventSleepDuringSeed),
	)
	return true
}

// SetTelemetryOptIn updates user consent for anonymous technical telemetry.
func (a *App) SetTelemetryOptIn(optIn bool) error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	cfg := a.cfgMgr.Get()
	cfg.General.TelemetryOptIn = optIn
	cfg.General.TelemetryPromptShown = true
	if err := a.cfgMgr.Save(cfg); err != nil {
		return err
	}
	if a.telemetryReporter != nil {
		a.telemetryReporter.SetOptIn(optIn)
	}
	return nil
}

// GetTelemetryPreview returns current sanitized telemetry payload for user inspection.
func (a *App) GetTelemetryPreview() (telemetry.Payload, error) {
	if a.telemetryReporter == nil {
		return telemetry.Payload{}, errors.New("telemetry reporter not initialized")
	}
	return a.telemetryReporter.GetPreviewPayload()
}

func (a *App) handleSessionStateChange(session seed.Session) {
	if a.telemetryReporter == nil || !a.telemetryReporter.IsOptIn() {
		return
	}
	switch session.State {
	case seed.StateConnecting:
		a.telemetryReporter.RecordSample(telemetry.CollectDynamicSample(telemetry.CheckpointT0))
	case seed.StateActive:
		a.telemetryReporter.RecordSample(telemetry.CollectDynamicSample(telemetry.CheckpointPostLaunch))
	case seed.StateCompleted, seed.StateTimedOut, seed.StateFailed, seed.StateStopped:
		a.dispatchTelemetryForSession(session)
	}
}

func (a *App) dispatchTelemetryForSession(session seed.Session) {
	if a.telemetryReporter == nil || !a.telemetryReporter.IsOptIn() {
		return
	}
	var currentRunID uint64
	if a.seeder != nil {
		currentRunID = a.seeder.CurrentRunID()
	}
	if currentRunID != 0 {
		if a.lastDispatchedRunID.Swap(currentRunID) == currentRunID {
			return
		}
	}

	sample := telemetry.CollectDynamicSample(telemetry.CheckpointExit)
	a.telemetryReporter.RecordSample(sample)

	var optErrors []string
	if session.ErrorMessage != "" {
		optErrors = append(optErrors, session.ErrorMessage)
	}

	a.telemetryReporter.SetDiagnostics(telemetry.SeedingDiagnostics{
		PotatoModeEnabled:   a.cfgMgr.Get().Potato.Enabled,
		FpsLimitTarget:      a.cfgMgr.Get().Potato.FpsLimit,
		LowResolutionActive: a.cfgMgr.Get().Potato.LowResolution,
		ExitReason:          string(session.State),
		OptimizationErrors:  optErrors,
	})

	a.telemetryWg.Add(1)
	go func(rep *telemetry.Reporter) {
		defer a.telemetryWg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := rep.Dispatch(ctx); err != nil {
			logger.Warn("Failed to deliver session telemetry report", "error", err)
		} else {
			rep.ResetSession()
		}
	}(a.telemetryReporter)
}

func (a *App) dispatchPreLaunchFailureTelemetry(err error) {
	if a.telemetryReporter == nil || !a.telemetryReporter.IsOptIn() {
		return
	}
	sample := telemetry.CollectDynamicSample(telemetry.CheckpointExit)
	a.telemetryReporter.RecordSample(sample)
	a.telemetryReporter.SetDiagnostics(telemetry.SeedingDiagnostics{
		PotatoModeEnabled:   a.cfgMgr.Get().Potato.Enabled,
		FpsLimitTarget:      a.cfgMgr.Get().Potato.FpsLimit,
		LowResolutionActive: a.cfgMgr.Get().Potato.LowResolution,
		ExitReason:          "failed",
		OptimizationErrors:  []string{err.Error()},
	})
	a.telemetryWg.Add(1)
	go func(rep *telemetry.Reporter) {
		defer a.telemetryWg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := rep.Dispatch(ctx); err != nil {
			logger.Warn("Failed to deliver pre-launch failure telemetry report", "error", err)
		} else {
			rep.ResetSession()
		}
	}(a.telemetryReporter)
}

// GetServers retrieves the fleet servers list
func (a *App) GetServers() ([]citadel.Server, error) {
	servers, err := a.citadel.GetServers(a.appContext())
	if err != nil {
		return nil, err
	}
	seedTarget := a.cfgMgr.Get().Seed.DefaultTargetPlayers
	for index := range servers {
		server := &servers[index]
		server.TargetSeedCount = seedTarget
		server.IsSeedingActive = (server.Status == "online" || server.Status == "seeding") && server.CurrentPlayers < seedTarget
		if server.IsSeedingActive {
			server.Status = "seeding"
		}
	}
	return servers, nil
}

// FetchServers provides an alias for GetServers matching frontend adapter expectations
func (a *App) FetchServers() ([]citadel.Server, error) {
	return a.GetServers()
}

// GetServerDetails returns details for a specific server
func (a *App) GetServerDetails(serverID string) (citadel.Server, error) {
	servers, err := a.GetServers()
	if err != nil {
		return citadel.Server{}, err
	}
	for _, s := range servers {
		if s.ID == serverID {
			return s, nil
		}
	}
	return citadel.Server{}, fmt.Errorf("server not found: %s", serverID)
}

func (a *App) logPreLaunchDiagnostics(mode string, plan *seed.Plan, server *citadel.Server) {
	potatoCfg := a.cfgMgr.Get().Potato
	cfgStatus := a.GetSquadConfigStatus()
	configState := fmt.Sprintf("isPotato=%t, currentFpsCap=%g", cfgStatus.IsPotatoMode, cfgStatus.CurrentFpsCap)
	if cfgStatus.ErrorMessage != "" {
		configState = "unknown (" + cfgStatus.ErrorMessage + ")"
	}
	steamRunning, steamError := a.platform.CheckSteamRunning()
	squadRunning, squadError := a.platform.CheckSquadRunning()

	lines := []string{
		fmt.Sprintf("Launch Mode:           %s", mode),
	}
	if server != nil {
		lines = append(lines,
			fmt.Sprintf("Target Server:         %s (ID: %s, Status: %s, Players: %d/%d)", server.Name, server.ID, server.Status, server.CurrentPlayers, server.MaxPlayers),
			fmt.Sprintf("Endpoint:              %s:%d (Query: %d)", server.ServerIP, server.GamePort, server.QueryPort),
		)
	}
	if plan != nil {
		lines = append(lines,
			fmt.Sprintf("Target Player Count:   %d players", plan.TargetPlayerCount),
			fmt.Sprintf("Max Duration:          %d minutes", plan.MaxDurationMinutes),
			fmt.Sprintf("Queue Server Limit:    %d", plan.ServerLimit),
			fmt.Sprintf("Post-Seed Action:      %s", plan.PostSeedAction),
			fmt.Sprintf("Timeout Action:        %s", plan.TimeoutAction),
		)
	}
	lines = append(lines,
		"--- Potato Mode Configuration ---",
		fmt.Sprintf("Potato Enabled:        %t", potatoCfg.Enabled),
		fmt.Sprintf("FPS Limit Target:      %d FPS", potatoCfg.FpsLimit),
		fmt.Sprintf("Low Resolution (480p): %t", potatoCfg.LowResolution),
		fmt.Sprintf("Low CPU Priority:      %t", potatoCfg.LowCpuPriority),
		fmt.Sprintf("Auto Restore On Exit:  %t", potatoCfg.AutoRestoreOnExit),
		"--- Squad & Environment Status ---",
		fmt.Sprintf("Squad Config Path:     %s", cfgStatus.ConfigPath),
		fmt.Sprintf("Config File Exists:    %t", cfgStatus.ConfigFound),
		fmt.Sprintf("Active Backup Exists:  %t (Backup: %s)", cfgStatus.HasBackup, cfgStatus.BackupPath),
		fmt.Sprintf("Current Config State:  %s", configState),
		fmt.Sprintf("Steam Process Running: %s", describeProcessState(steamRunning, steamError)),
		fmt.Sprintf("Squad Process Running: %s", describeProcessState(squadRunning, squadError)),
	)
	logger.Banner("PRE-LAUNCH CONFIGURATION DIAGNOSTICS", lines...)
}

func describeProcessState(isRunning bool, err error) string {
	if err != nil {
		return fmt.Sprintf("unknown (%v)", err)
	}
	return fmt.Sprint(isRunning)
}

// StartSeed resolves the visible fleet and starts a deterministic multi-server queue.
func (a *App) StartSeed(plan seed.Plan) (result seed.Session, startError error) {
	if a.isShuttingDown.Load() {
		return seed.Session{}, fmt.Errorf("Citadel Sentry is shutting down")
	}
	if a.updateInProgress.Load() {
		return seed.Session{}, fmt.Errorf("Citadel Sentry update is in progress")
	}
	fleet, err := a.citadel.GetServers(a.appContext())
	if err != nil {
		logger.Error("Failed to load Citadel server fleet for seeding", "error", err)
		return seed.Session{}, fmt.Errorf("load Citadel server fleet: %w", err)
	}
	queue, err := seed.BuildQueue(plan, fleet)
	if err != nil {
		logger.Error("Failed to build seed queue", "error", err)
		return seed.Session{}, err
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	isPotatoPrepared := false
	if a.directJoinCancel != nil {
		return seed.Session{}, fmt.Errorf("a direct Squad join is still pending; stop it before starting seeding")
	}
	defer func() {
		if startError != nil && isPotatoPrepared {
			if restoreError := a.restoreSquadIfStopped(); restoreError != nil {
				startError = errors.Join(startError, fmt.Errorf("restore failed preparation: %w", restoreError))
			}
		}
	}()
	if a.isShuttingDown.Load() {
		return seed.Session{}, fmt.Errorf("Citadel Sentry is shutting down")
	}
	if a.updateInProgress.Load() {
		return seed.Session{}, fmt.Errorf("Citadel Sentry update is in progress")
	}

	var firstServer *citadel.Server
	if len(queue) > 0 {
		firstServer = &queue[0]
	}
	a.logPreLaunchDiagnostics("Seed Mission Queue", &plan, firstServer)
	a.lastDispatchedRunID.Store(0)

	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() {
		return seed.Session{}, fmt.Errorf("stop the current seed session before starting another")
	}
	if err := a.seeder.CancelPowerAction(); err != nil {
		return seed.Session{}, err
	}
	potatoCfg := a.cfgMgr.Get().Potato
	if err := a.platform.RestoreProcessPriority(); err != nil && !isPriorityAccessDenied(err) {
		return seed.Session{}, fmt.Errorf("restore previous process priority before seeding: %w", err)
	}
	if potatoCfg.Enabled && potatoCfg.LowCpuPriority {
		if err := a.platform.CheckProcessPrioritySupport(); err != nil {
			logger.Warn("Optional process priority optimization is unavailable; continuing seeding", "error", err)
			potatoCfg.LowCpuPriority = false // Skip for this run; do not rewrite user preferences.
		}
	}
	if potatoCfg.Enabled {
		if !a.isPreparingPotato.CompareAndSwap(false, true) {
			return seed.Session{}, fmt.Errorf("Potato Mode is already being prepared")
		}
		defer a.isPreparingPotato.Store(false)

		isRunning, err := a.platform.CheckSquadRunning()
		if err != nil {
			return seed.Session{}, fmt.Errorf("verify Squad process state: %w", err)
		}
		if isRunning {
			matches := false
			if a.squadConfig != nil {
				matches, _ = a.squadConfig.CheckMatchesPotatoSettings(potatoCfg.FpsLimit, potatoCfg.LowResolution)
			}
			if matches {
				logger.Info("Squad is already running with matching Potato Mode settings; proceeding without restart")
			} else if plan.ForceRunningSettings {
				logger.Warn("Squad is running with different settings; forced start requested, proceeding without restart")
			} else {
				return seed.Session{}, fmt.Errorf("close Squad normally before changing its settings")
			}
		} else {
			if err := a.restoreSquadIfStopped(); err != nil {
				return seed.Session{}, fmt.Errorf("restore previous Potato Mode session: %w", err)
			}

			if a.squadConfig != nil {
				a.potatoRunID = 0
				logger.Info("Applying potato mode config before launching seed (startup at 60 FPS for fast loading)",
					"targetFps", potatoCfg.FpsLimit,
					"lowResolution", potatoCfg.LowResolution,
					"lowCpuPriority", potatoCfg.LowCpuPriority,
				)
				gpuInfo := a.platform.DetectGPU()
				// Start Squad with 60 FPS so loading screens finish in seconds instead of minutes.
				// Once connected in-game, connectionHook clamps FPS to potatoCfg.FpsLimit.
				startupFps := 60
				if err := a.squadConfig.ApplyPotatoConfig(startupFps, potatoCfg.LowResolution, gpuInfo); err != nil {
					a.dispatchPreLaunchFailureTelemetry(err)
					return seed.Session{}, fmt.Errorf("apply Potato Mode before launching Squad: %w", err)
				}
				isPotatoPrepared = true
				logger.Info("Potato Mode settings prepared with fast startup FPS before Squad launch")
				if a.ctx != nil {
					wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
				}
			}
			if err := a.requireSquadStopped(); err != nil {
				return seed.Session{}, fmt.Errorf("verify Squad stayed closed during Potato Mode preparation: %w", err)
			}
		}
	} else {
		if a.squadConfig != nil && a.squadConfig.HasActiveBackup() {
			isRunning, _ := a.platform.CheckSquadRunning()
			if isRunning {
				if !plan.ForceRunningSettings {
					if err := a.requireSquadStopped(); err != nil {
						return seed.Session{}, err
					}
				}
			} else {
				if err := a.squadConfig.RestoreOriginalConfig(); err != nil {
					return seed.Session{}, fmt.Errorf("restore Squad settings before normal seeding: %w", err)
				}
			}
		}
		logger.Info("Potato mode is disabled; launching with the player's current Squad settings")
	}

	if err := a.seeder.Start(a.appContext(), plan, queue); err != nil {
		logger.Error("Failed to start seeder engine", "error", err)
		a.dispatchPreLaunchFailureTelemetry(err)
		if potatoCfg.Enabled {
			if restoreErr := a.restoreSquadIfStopped(); restoreErr != nil {
				return seed.Session{}, fmt.Errorf("start seed: %w; restore Potato Mode: %v", err, restoreErr)
			}
		}
		return seed.Session{}, err
	}

	if potatoCfg.Enabled && potatoCfg.LowCpuPriority {
		go a.manageSquadPriority(a.appContext(), a.seeder.CurrentRunID())
	}

	if potatoCfg.Enabled {
		a.potatoRunID = a.seeder.CurrentRunID()
	}
	session := a.seeder.GetSession()
	if a.telemetryReporter != nil && a.telemetryReporter.IsOptIn() {
		a.telemetryReporter.RecordSample(telemetry.CollectDynamicSample(telemetry.CheckpointT0))
	}
	logger.Info("Seed mission successfully launched", "state", session.State, "queueCount", len(queue))
	return session, nil
}

// isPriorityAccessDenied checks if priority modification was blocked by OS permissions or EAC.
func isPriorityAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, platform.ErrPriorityAccessDenied) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "access denied")
}

// manageSquadPriority observes each game process throughout the owned queue.
func (a *App) manageSquadPriority(ctx context.Context, runID uint64) {
	ticker := time.NewTicker(priorityPollInterval)
	defer ticker.Stop()
	startedAt := time.Now()
	hasObservedGame := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isApplied, isCurrent, err := a.applySquadPriorityForRun(runID)
			if !isCurrent {
				return
			}
			if err != nil {
				logger.Warn("Squad CPU priority polling halted due to error; continuing seeding with normal priority", "error", err)
				return
			}
			if isApplied {
				hasObservedGame = true
				return
			}
			if !hasObservedGame && time.Since(startedAt) >= priorityStartupTimeout {
				// Priority adjustment is an optional optimization, not a seeding
				// prerequisite. Startup detection delays must not abort a live run.
				logger.Warn("Squad was not observed before priority optimization timeout; continuing seeding normally")
				return
			}
		}
	}
}

// applySquadPriorityForRun prevents an old polling task from changing a later game.
// CPU priority optimization is strictly optional and must never abort a live seeding session.
func (a *App) applySquadPriorityForRun(runID uint64) (isApplied bool, isCurrent bool, err error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if a.isShuttingDown.Load() {
		return false, false, nil
	}
	isCurrent, err = a.seeder.PerformRunOperation(runID, func() error {
		isRunning, err := a.platform.CheckSquadRunning()
		if err != nil {
			logger.Warn("Unable to verify Squad process for CPU priority adjustment; continuing seeding normally", "error", err)
			return nil
		}
		if !isRunning {
			return nil
		}
		if err := a.platform.SetProcessLowPriority(); err != nil {
			if errors.Is(err, platform.ErrPriorityProcessPending) {
				return nil
			}
			if isPriorityAccessDenied(err) {
				logger.Warn("Squad CPU priority reduction skipped: restricted by Easy Anti-Cheat or OS permissions; continuing with normal priority", "error", err)
				isApplied = true
				return nil
			}
			logger.Warn("Squad CPU priority adjustment skipped due to unexpected platform error; continuing with normal priority", "error", err)
			isApplied = true
			return nil
		}
		isApplied = true
		return nil
	})
	return isApplied, isCurrent, err
}

// StartSeedSession is retained for compatibility with earlier generated bindings.
func (a *App) StartSeedSession(plan seed.Plan, _ citadel.Server) error {
	_, err := a.StartSeed(plan)
	return err
}

// StopSeed aborts the active seeding mission and returns its terminal snapshot.
func (a *App) StopSeed() (seed.Session, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if a.directJoinCancel != nil {
		a.directJoinCancel()
	}
	if a.seeder == nil {
		return seed.Session{}, fmt.Errorf("seed engine is not initialized")
	}
	logger.Banner("STOPPING SEED MISSION",
		"Trigger: UI stop request",
		"Action: terminating active queue and restoring graphics configuration...",
	)
	cancelErr := a.seeder.CancelPowerAction()
	if err := a.seeder.Stop(); err != nil {
		logger.Warn("Seeder engine returned warning on stop", "error", err)
		return a.seeder.GetSession(), errors.Join(err, cancelErr)
	}
	priorityError := a.platform.RestoreProcessPriority()
	if isPriorityAccessDenied(priorityError) {
		logger.Warn("Squad CPU priority restoration skipped: restricted by permissions", "error", priorityError)
		priorityError = nil
	}
	if a.squadConfig != nil && a.squadConfig.HasActiveBackup() && a.cfgMgr.Get().Potato.AutoRestoreOnExit {
		if err := a.restoreSquadIfStopped(); err != nil {
			logger.Error("Could not restore Squad settings after stopping seed", "error", err)
		}
	}
	session := a.seeder.GetSession()
	a.dispatchTelemetryForSession(session)
	logger.Info("Seed mission stopped", "state", session.State)
	return session, errors.Join(cancelErr, priorityError)
}

// CancelPowerAction aborts any impending power action countdown (sleep or shutdown) and Windows shutdown command.
func (a *App) CancelPowerAction() error {
	logger.Info("CancelPowerAction requested from UI")
	if a.seeder != nil {
		if err := a.seeder.CancelPowerAction(); err != nil {
			return err
		}
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "citadel:power_action_cancelled", true)
	}
	return nil
}

// StopSeedSession is retained for compatibility with earlier generated bindings.
func (a *App) StopSeedSession() error {
	_, err := a.StopSeed()
	return err
}

// GetSeedSession returns the real-time seeding session telemetry
func (a *App) GetSeedSession() seed.Session {
	return a.seeder.GetSession()
}

// SkipToNextServer skips seeding the currently active server and advances to the next server in queue.
func (a *App) SkipToNextServer() (bool, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if a.seeder == nil {
		return false, fmt.Errorf("seeder engine is not initialized")
	}
	if !a.seeder.GetSession().State.IsActive() {
		return false, fmt.Errorf("no active seed session to skip")
	}
	logger.Info("User requested skipping to next server in queue")
	if err := a.seeder.SkipCurrentServer(); err != nil {
		return false, err
	}
	return true, nil
}

type RunningSquadState struct {
	IsRunning     bool `json:"isRunning"`
	MatchesPotato bool `json:"matchesPotato"`
	CurrentFps    int  `json:"currentFps"`
}

// CheckRunningSquadPotatoState checks if Squad is running and whether its configuration already matches Potato Mode.
func (a *App) CheckRunningSquadPotatoState() (RunningSquadState, error) {
	isRunning, err := a.platform.CheckSquadRunning()
	if err != nil {
		return RunningSquadState{}, fmt.Errorf("check squad process: %w", err)
	}
	if !isRunning || a.squadConfig == nil {
		return RunningSquadState{IsRunning: isRunning}, nil
	}
	potatoCfg := a.cfgMgr.Get().Potato
	matches, err := a.squadConfig.CheckMatchesPotatoSettings(potatoCfg.FpsLimit, potatoCfg.LowResolution)
	if err != nil {
		return RunningSquadState{IsRunning: true, MatchesPotato: false}, nil
	}
	status := a.squadConfig.GetStatus()
	return RunningSquadState{
		IsRunning:     true,
		MatchesPotato: matches,
		CurrentFps:    int(status.CurrentFpsCap),
	}, nil
}

// RestartSquadWithPotato closes the running Squad process, applies potato configuration, and starts the seed mission.
func (a *App) RestartSquadWithPotato(plan seed.Plan) (seed.Session, error) {
	a.operationMu.Lock()
	if a.gameCtrl != nil {
		logger.Info("Closing Squad process to apply Potato Mode and restart seeding")
		_ = a.gameCtrl.CloseSquad()
	}
	a.operationMu.Unlock()

	// Wait up to 10 seconds for Squad process to exit
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		running, err := a.platform.CheckSquadRunning()
		if err == nil && !running {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	return a.StartSeed(plan)
}

// JoinServerDirect launches Squad through the website's SquadBrowser resolver.
func (a *App) JoinServerDirect(serverID string) (bool, error) {
	if a.isShuttingDown.Load() {
		return false, fmt.Errorf("Citadel Sentry is shutting down")
	}
	a.operationMu.Lock()
	isLocked := true
	defer func() {
		if isLocked {
			a.operationMu.Unlock()
		}
	}()
	if a.directJoinCancel != nil {
		return false, fmt.Errorf("a direct Squad join is already pending")
	}
	if a.isShuttingDown.Load() {
		return false, fmt.Errorf("Citadel Sentry is shutting down")
	}
	if a.updateInProgress.Load() {
		return false, fmt.Errorf("Citadel Sentry update is in progress")
	}
	logger.Info("Direct join server requested from UI", "serverID", serverID)
	if a.seeder != nil && a.seeder.GetSession().State.IsActive() {
		logger.Info("Stopping active seed before direct server join")
		if err := a.seeder.Stop(); err != nil {
			return false, fmt.Errorf("stop seeding before direct join: %w", err)
		}
	}
	if a.seeder != nil && a.seeder.HasRunningWorker() {
		return false, fmt.Errorf("seeding is still stopping; retry direct join after it finishes")
	}
	if err := a.platform.RestoreProcessPriority(); err != nil && !isPriorityAccessDenied(err) {
		return false, fmt.Errorf("restore Squad CPU priority before direct join: %w", err)
	}
	isRunning, _ := a.platform.CheckSquadRunning()
	if !isRunning {
		// If Squad is stopped and potato mode backup is active, restore original user settings first so user plays in full quality!
		if a.squadConfig != nil && a.squadConfig.HasActiveBackup() {
			logger.Info("Restoring original Squad config before direct join to play")
			if err := a.squadConfig.RestoreOriginalConfig(); err != nil {
				return false, fmt.Errorf("restore Squad settings before direct join: %w", err)
			}
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
			}
		}
	} else {
		logger.Info("Squad is already running; switching server directly via lobby invitation")
	}

	var srv *citadel.Server
	if servers, err := a.GetServers(); err == nil {
		for _, s := range servers {
			if s.ID == serverID {
				srv = &s
				break
			}
		}
	}
	a.logPreLaunchDiagnostics("Direct Join (Full Quality)", nil, srv)

	joinCtx, cancel := context.WithCancel(a.appContext())
	a.directJoinID++
	joinID := a.directJoinID
	a.directJoinCancel = cancel
	isLocked = false
	a.operationMu.Unlock()
	defer func() {
		cancel()
		a.operationMu.Lock()
		if a.directJoinID == joinID {
			a.directJoinCancel = nil
		}
		a.operationMu.Unlock()
	}()
	resolve := func(ctx context.Context) (string, error) { return a.citadel.ResolveJoinURL(ctx, serverID) }
	if err := a.gameCtrl.ConnectResolved(joinCtx, resolve, false); err != nil {
		logger.Error("Failed to launch Squad for direct join", "serverID", serverID, "error", err)
		return false, fmt.Errorf("launch Squad: %w", err)
	}

	targetServer := citadel.Server{ID: serverID}
	if srv != nil {
		targetServer = *srv
	}

	// Wait briefly to observe whether Squad immediately rejects the invitation
	// (e.g. incompatible lobby build, missing EOS room ID, or busy session search).
	checkTicker := time.NewTicker(100 * time.Millisecond)
	defer checkTicker.Stop()
	checkTimeout := time.After(3 * time.Second)
waitInvite:
	for {
		select {
		case <-joinCtx.Done():
			return false, joinCtx.Err()
		case <-checkTimeout:
			break waitInvite
		case <-checkTicker.C:
			connected, checkErr := a.gameCtrl.CheckServerConnection(targetServer)
			if checkErr != nil {
				logger.Error("Squad rejected direct join invitation", "serverID", serverID, "error", checkErr)
				return false, fmt.Errorf("join invitation rejected: %w", checkErr)
			}
			if connected {
				logger.Info("Direct join in-game connection confirmed", "serverID", serverID)
				return true, nil
			}
		}
	}

	logger.Info("Direct join URI dispatched to Steam; in-game connection is not confirmed", "serverID", serverID)
	return true, nil
}

// DirectSteamConnect launches Squad directly to a Citadel server using steam://connect/<ip>:<game_port>, bypassing EOS lobby.
func (a *App) DirectSteamConnect(serverID string) error {
	info, ok := game.KnownCitadelServers[serverID]
	if !ok {
		return fmt.Errorf("unknown Citadel server %s", serverID)
	}
	uri := fmt.Sprintf("steam://connect/%s:%d", game.CitadelHostIP, info.GamePort)
	logger.Info("Direct Steam Connect requested", "serverID", serverID, "uri", uri)
	return a.platform.LaunchURI(a.appContext(), uri)
}

// GetConsoleConnectCommand returns the direct in-game console command, e.g. "connect 62.122.215.132:7787".
func (a *App) GetConsoleConnectCommand(serverID string) string {
	info, ok := game.KnownCitadelServers[serverID]
	if !ok {
		return fmt.Sprintf("connect %s:7787", game.CitadelHostIP)
	}
	return fmt.Sprintf("connect %s:%d", game.CitadelHostIP, info.GamePort)
}

// GetPotatoSettings returns the current potato mode configuration.
func (a *App) GetPotatoSettings() config.PotatoSettings {
	return a.cfgMgr.Get().Potato
}

// SavePotatoSettings persists updated potato mode preferences.
func (a *App) SavePotatoSettings(settings config.PotatoSettings) error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return err
	}
	logger.Info("User updated Potato Mode configuration",
		"enabled", settings.Enabled,
		"fpsLimit", settings.FpsLimit,
		"lowResolution", settings.LowResolution,
		"lowCpuPriority", settings.LowCpuPriority,
		"autoRestoreOnExit", settings.AutoRestoreOnExit,
	)
	current := a.cfgMgr.Get()
	current.Potato = settings
	if err := a.cfgMgr.Save(current); err != nil {
		logger.Error("Failed to save updated Potato Mode settings", "error", err)
		return err
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.GetSquadConfigStatus())
	}
	return nil
}

// SaveCurrentSquadConfigAsBackup saves the player's current GameUserSettings.ini as the official baseline backup.
func (a *App) SaveCurrentSquadConfigAsBackup() (bool, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return false, err
	}
	if a.squadConfig == nil {
		return false, fmt.Errorf("squad config manager not initialized")
	}
	if err := a.requireSquadStopped(); err != nil {
		return false, err
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() {
		return false, fmt.Errorf("close Squad and stop seeding before saving its settings backup")
	}
	err := a.squadConfig.SaveCurrentAsBackup(false)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
	}
	return err == nil, err
}

// RestoreSquadConfig manually restores the player's original GameUserSettings.ini from backup.
func (a *App) RestoreSquadConfig() (bool, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return false, err
	}
	if a.squadConfig == nil {
		return false, fmt.Errorf("squad config manager not initialized")
	}
	if err := a.requireSquadStopped(); err != nil {
		return false, err
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() || a.isPreparingPotato.Load() {
		return false, fmt.Errorf("close Squad and stop seeding before restoring its settings")
	}
	err := errors.Join(a.platform.RestoreProcessPriority(), a.squadConfig.RestoreOriginalConfig())
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
	}
	return err == nil, err
}

// GetGPUInfo returns information about the detected system graphics card.
func (a *App) GetGPUInfo() platform.GPUInfo {
	if a.platform != nil {
		return a.platform.DetectGPU()
	}
	return platform.DetectGPU()
}

// RestoreInitialBaseline restores Squad settings to the permanent first-launch baseline.
func (a *App) RestoreInitialBaseline() (bool, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return false, err
	}
	if a.squadConfig == nil {
		return false, fmt.Errorf("squad config manager not initialized")
	}
	if err := a.requireSquadStopped(); err != nil {
		return false, err
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() || a.isPreparingPotato.Load() {
		return false, fmt.Errorf("close Squad and stop seeding before restoring its settings")
	}
	err := errors.Join(a.platform.RestoreProcessPriority(), a.squadConfig.RestoreInitialBaseline())
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
	}
	return err == nil, err
}

// HasInitialBaseline checks if the first-launch baseline is captured.
func (a *App) HasInitialBaseline() bool {
	if a.squadConfig == nil {
		return false
	}
	return a.squadConfig.HasInitialBaseline()
}

// CreateRestorePoint creates a named snapshot of current Squad settings.
func (a *App) CreateRestorePoint(name, description string) (game.RestorePoint, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return game.RestorePoint{}, err
	}
	if a.squadConfig == nil {
		return game.RestorePoint{}, fmt.Errorf("squad config manager not initialized")
	}
	if err := a.requireSquadStopped(); err != nil {
		return game.RestorePoint{}, err
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() || a.isPreparingPotato.Load() {
		return game.RestorePoint{}, fmt.Errorf("close Squad and stop seeding before creating a restore point")
	}
	rp, err := a.squadConfig.CreateRestorePoint(name, description)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
	}
	return rp, err
}

// ListRestorePoints returns all saved snapshots sorted by date.
func (a *App) ListRestorePoints() ([]game.RestorePoint, error) {
	if a.squadConfig == nil {
		return []game.RestorePoint{}, fmt.Errorf("squad config manager not initialized")
	}
	return a.squadConfig.ListRestorePoints()
}

// RestoreRestorePoint restores settings from a specific named snapshot.
func (a *App) RestoreRestorePoint(id string) (bool, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return false, err
	}
	if a.squadConfig == nil {
		return false, fmt.Errorf("squad config manager not initialized")
	}
	if err := a.requireSquadStopped(); err != nil {
		return false, err
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() || a.isPreparingPotato.Load() {
		return false, fmt.Errorf("close Squad and stop seeding before restoring a snapshot")
	}
	err := errors.Join(a.platform.RestoreProcessPriority(), a.squadConfig.RestoreRestorePoint(id))
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
	}
	return err == nil, err
}

// DeleteRestorePoint deletes a named snapshot.
func (a *App) DeleteRestorePoint(id string) (bool, error) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if err := a.requireStateChangesAllowed(); err != nil {
		return false, err
	}
	if a.squadConfig == nil {
		return false, fmt.Errorf("squad config manager not initialized")
	}
	err := a.squadConfig.DeleteRestorePoint(id)
	return err == nil, err
}

// GetSquadConfigStatus returns current diagnostic status of Squad graphics and backup.
func (a *App) GetSquadConfigStatus() game.SquadConfigStatus {
	if a.squadConfig == nil {
		return game.SquadConfigStatus{ErrorMessage: "Squad config manager is unavailable"}
	}
	return a.squadConfig.GetStatus()
}

// requireStateChangesAllowed protects state after update or shutdown takes ownership.
func (a *App) requireStateChangesAllowed() error {
	if a.isShuttingDown.Load() {
		return fmt.Errorf("Citadel Sentry is shutting down")
	}
	if a.updateInProgress.Load() {
		return fmt.Errorf("Citadel Sentry update is in progress")
	}
	return nil
}

// requireSquadStopped permits config changes only after a successful process check.
func (a *App) requireSquadStopped() error {
	isRunning, err := a.platform.CheckSquadRunning()
	if err != nil {
		return fmt.Errorf("verify Squad process state: %w", err)
	}
	if isRunning {
		return fmt.Errorf("close Squad normally before changing its settings")
	}
	return nil
}

// restoreSquadIfStopped finishes a pending potato transaction after Squad exits.
func (a *App) restoreSquadIfStopped() error {
	if a.squadConfig == nil || !a.squadConfig.HasActiveBackup() {
		return nil
	}
	if a.seeder.HasRunningWorker() || a.seeder.GetSession().State.IsActive() {
		return nil
	}
	isRunning, err := a.platform.CheckSquadRunning()
	if err != nil {
		return fmt.Errorf("verify Squad exited before restoration: %w", err)
	}
	if isRunning {
		return nil
	}
	if err := a.squadConfig.RestoreOriginalConfig(); err != nil {
		return err
	}
	if a.ctx != nil && !a.isShuttingDown.Load() {
		wruntime.EventsEmit(a.ctx, "squad:config_status", a.squadConfig.GetStatus())
	}
	return nil
}

// restoreSquadAutomaticallyIfStopped rolls back failed missions regardless of exit preferences.
func (a *App) restoreSquadAutomaticallyIfStopped() error {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if a.isPreparingPotato.Load() || a.seeder.HasRunningWorker() {
		return nil
	}
	session := a.seeder.GetSession()
	if session.State.IsActive() {
		return nil
	}
	priorityError := a.platform.RestoreProcessPriority()
	if isPriorityAccessDenied(priorityError) {
		logger.Warn("Squad CPU priority restoration skipped: restricted by permissions", "error", priorityError)
		priorityError = nil
	}
	if a.squadConfig == nil {
		return priorityError
	}
	if a.potatoRunID != 0 && a.potatoRunID == a.seeder.CurrentRunID() {
		switch session.State {
		case seed.StateCompleted, seed.StateTimedOut, seed.StateStopped:
			if session.HasObservedGame {
				if err := a.squadConfig.MarkSessionFinished(); err != nil {
					return errors.Join(priorityError, err)
				}
			}
		}
		a.potatoRunID = 0
	}
	shouldRestore, err := a.squadConfig.ShouldRestoreAutomatically(a.cfgMgr.Get().Potato.AutoRestoreOnExit)
	if err != nil || !shouldRestore {
		return errors.Join(priorityError, err)
	}
	return errors.Join(priorityError, a.restoreSquadIfStopped())
}

// watchSquadRestore recovers settings when a seed session has ended and Squad exits.
func (a *App) watchSquadRestore(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastError := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.isPreparingPotato.Load() || a.squadConfig == nil {
				continue
			}
			err := a.restoreSquadAutomaticallyIfStopped()
			if err != nil {
				if err.Error() != lastError {
					logger.Error("Pending Squad settings restoration failed", "error", err)
					lastError = err.Error()
				}
			} else {
				lastError = ""
			}
		}
	}
}

func (a *App) appContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// GetSystemStatus returns diagnostic information for bridge & game processes
func (a *App) GetSystemStatus() map[string]any {
	squadRunning, squadError := a.platform.CheckSquadRunning()
	steamRunning, steamError := a.platform.CheckSteamRunning()
	var squadStatus, steamStatus any
	var processErrors []error
	if squadError == nil {
		squadStatus = squadRunning
	} else {
		processErrors = append(processErrors, fmt.Errorf("Squad: %w", squadError))
	}
	if steamError == nil {
		steamStatus = steamRunning
	} else {
		processErrors = append(processErrors, fmt.Errorf("Steam: %w", steamError))
	}
	processStatusError := ""
	if err := errors.Join(processErrors...); err != nil {
		processStatusError = err.Error()
	}
	cfgStatus := map[string]any{}
	if a.squadConfig != nil {
		st := a.squadConfig.GetStatus()
		cfgStatus = map[string]any{
			"configFound":       st.ConfigFound,
			"configPath":        st.ConfigPath,
			"hasBackup":         st.HasBackup,
			"hasPendingRestore": st.HasPendingRestore,
			"backupPath":        st.BackupPath,
			"isPotatoMode":      st.IsPotatoMode,
			"currentFpsCap":     st.CurrentFpsCap,
			"errorMessage":      st.ErrorMessage,
		}
	}
	return map[string]any{
		"canSetLowCpuPriority": a.platform.CheckProcessPrioritySupport() == nil,
		"platform":             a.platform.Name(),
		"appVersion":           applicationVersion(),
		"squadRunning":         squadStatus,
		"steamRunning":         steamStatus,
		"processStatusError":   processStatusError,
		"lastPingAt":           time.Now().Format(time.RFC3339),
		"logFilePath":          logger.GetLogFilePath(),
		"squadConfigStatus":    cfgStatus,
	}
}

// GetBridgeStatus is a direct alias for GetSystemStatus to ensure full bridge compatibility.
func (a *App) GetBridgeStatus() map[string]any {
	return a.GetSystemStatus()
}

// GetLogFilePath returns the absolute path to the active log file on disk
func (a *App) GetLogFilePath() string {
	return logger.GetLogFilePath()
}

// GetRecentLogs returns recent lines from the active log file for in-app diagnostics.
func (a *App) GetRecentLogs(limit int) ([]string, error) {
	return logger.GetRecentLines(limit)
}

// ClearLogs clears the active log file on disk.
func (a *App) ClearLogs() error {
	logger.Info("ClearLogs requested from UI")
	return logger.Clear()
}

// OpenLogsFolder opens the user's OS file manager highlighting or inside the logs directory
func (a *App) OpenLogsFolder() (string, error) {
	logPath := logger.GetLogFilePath()
	if logPath == "" {
		return "", fmt.Errorf("log file path not available")
	}
	logger.Info("Opening logs folder", "path", logPath)
	if err := a.platform.OpenFolder(logPath); err != nil {
		logger.Error("Failed to open logs folder", "error", err, "path", logPath)
		return logPath, err
	}
	return logPath, nil
}

// OpenExternal opens a URL in user's default browser
func (a *App) OpenExternal(url string) error {
	logger.Info("Opening external URL", "url", url)
	return a.platform.OpenBrowser(url)
}

// Window control bindings
func (a *App) MinimizeWindow() {
	if a.ctx != nil {
		wruntime.WindowMinimise(a.ctx)
	}
}

func (a *App) MaximizeWindow() {
	if a.ctx != nil {
		wruntime.WindowToggleMaximise(a.ctx)
	}
}

func (a *App) CloseWindow() {
	if err := a.restoreSquadAutomaticallyIfStopped(); err != nil {
		logger.Error("Could not restore Squad settings on application close", "error", err)
	}
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}
