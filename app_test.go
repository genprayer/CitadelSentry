package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/config"
	"citadel-seeder/internal/game"
	"citadel-seeder/internal/platform"
	"citadel-seeder/internal/seed"
	"citadel-seeder/internal/telemetry"
)

func TestUnavailableConfigManagerReportsUnknownStatus(t *testing.T) {
	if status := (&App{}).GetSquadConfigStatus(); status.ErrorMessage == "" {
		t.Fatal("unavailable config manager was shown as an ordinary missing game config")
	}
}

func TestAppRejectsMutationsDuringUpdateOrShutdown(t *testing.T) {
	for _, lifecycle := range []string{"update", "shutdown"} {
		for _, operation := range []string{"settings", "potato", "backup", "restore"} {
			t.Run(lifecycle+"/"+operation, func(t *testing.T) {
				app, _, _ := newTestApp(t)
				before := app.cfgMgr.Get()
				if operation == "restore" {
					if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
						t.Fatal(err)
					}
				}
				gameBefore, err := os.ReadFile(app.squadConfig.ConfigPath())
				if err != nil {
					t.Fatal(err)
				}
				if lifecycle == "update" {
					app.updateInProgress.Store(true)
				} else {
					app.isShuttingDown.Store(true)
				}
				switch operation {
				case "settings":
					settings := before
					settings.Seed.DefaultTargetPlayers++
					if app.SaveSettings(settings) {
						t.Fatal("blocked lifecycle accepted application settings")
					}
				case "potato":
					settings := before.Potato
					settings.FpsLimit = 5
					if err := app.SavePotatoSettings(settings); err == nil {
						t.Fatal("blocked lifecycle accepted potato settings")
					}
				case "backup":
					if ok, err := app.SaveCurrentSquadConfigAsBackup(); err == nil || ok {
						t.Fatal("blocked lifecycle replaced the game baseline")
					}
					if _, err := os.Stat(app.squadConfig.BackupPath()); !os.IsNotExist(err) {
						t.Fatal("blocked backup created a record")
					}
				case "restore":
					if ok, err := app.RestoreSquadConfig(); err == nil || ok {
						t.Fatal("blocked lifecycle rewrote the game settings")
					}
					if !app.squadConfig.HasActiveBackup() {
						t.Fatal("blocked restore discarded its obligation")
					}
				}
				if app.cfgMgr.Get() != before {
					t.Fatal("blocked operation changed cached settings")
				}
				gameAfter, err := os.ReadFile(app.squadConfig.ConfigPath())
				if err != nil || !bytes.Equal(gameBefore, gameAfter) {
					t.Fatal("blocked operation changed the game's bytes")
				}
			})
		}
	}
}

type testAppPlatform struct {
	isSquadRunning       atomic.Bool
	shutdownCancels      atomic.Int64
	priorityChanges      atomic.Int64
	sleepAcquires        atomic.Int64
	sleepReleases        atomic.Int64
	processError         error
	priorityError        error
	priorityRestoreError error
	priorityRestores     atomic.Int64
	canLaunch            bool
	onLaunch             func(string) error
}

func (*testAppPlatform) Name() string           { return "test" }
func (p *testAppPlatform) IsSquadRunning() bool { return p.isSquadRunning.Load() }
func (p *testAppPlatform) CheckSquadRunning() (bool, error) {
	return p.isSquadRunning.Load(), p.processError
}
func (*testAppPlatform) IsSteamRunning() bool               { return true }
func (p *testAppPlatform) CheckSteamRunning() (bool, error) { return true, p.processError }
func (p *testAppPlatform) PreventSleep(context.Context, string) (platform.ReleaseFunc, error) {
	p.sleepAcquires.Add(1)
	var once sync.Once
	return func() { once.Do(func() { p.sleepReleases.Add(1) }) }, nil
}
func (*testAppPlatform) ExecutePowerAction(string) error {
	return errors.New("native power actions are forbidden in tests")
}
func (p *testAppPlatform) CancelPowerAction() error { p.shutdownCancels.Add(1); return nil }
func (*testAppPlatform) OpenBrowser(string) error   { return nil }
func (*testAppPlatform) OpenFolder(string) error    { return nil }
func (p *testAppPlatform) LaunchURI(_ context.Context, uri string) error {
	if p.onLaunch != nil {
		return p.onLaunch(uri)
	}
	if p.canLaunch {
		return nil
	}
	return errors.New("native game launch is forbidden in tests")
}
func (*testAppPlatform) CloseGame() error { return nil }
func (p *testAppPlatform) SetProcessLowPriority(...string) error {
	p.priorityChanges.Add(1)
	return p.priorityError
}

func (*testAppPlatform) CheckProcessPrioritySupport() error { return nil }
func (*testAppPlatform) DetectGPU() platform.GPUInfo {
	return platform.GPUInfo{Vendor: platform.GPUVendorNVIDIA, Model: "Test GeForce RTX 4080", IsRTX: true}
}
func (p *testAppPlatform) RestoreProcessPriority() error {
	p.priorityRestores.Add(1)
	return p.priorityRestoreError
}

func TestAppPriorityManagerCannotModifyAGameOutsideItsRun(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	adapter.isSquadRunning.Store(true)
	isApplied, isCurrent, err := app.applySquadPriorityForRun(app.seeder.CurrentRunID())
	if err != nil || isApplied || isCurrent || adapter.priorityChanges.Load() != 0 {
		t.Fatalf("inactive seed task changed a player's game priority: applied=%t current=%t error=%v", isApplied, isCurrent, err)
	}
}

func TestAppWaitsForPriorityProcessWithoutReportingSuccessOrFailure(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.seeder = seed.NewEngine(adapter, app.gameCtrl, testPendingFleet{}, app.cfgMgr, nil)
	server := citadel.Server{ID: "1", Status: "online", CurrentPlayers: 10, MaxPlayers: 100}
	plan := seed.Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: seed.PostSeedDoNothing}
	if err := app.seeder.Start(ctx, plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	adapter.isSquadRunning.Store(true)
	adapter.priorityError = platform.ErrPriorityProcessPending
	isApplied, isCurrent, err := app.applySquadPriorityForRun(app.seeder.CurrentRunID())
	if err != nil || isApplied || !isCurrent || !app.seeder.GetSession().State.IsActive() {
		t.Fatalf("pending game process did not remain an honest wait: applied=%t current=%t error=%v", isApplied, isCurrent, err)
	}
	if err := app.seeder.Stop(); err != nil {
		t.Fatal(err)
	}
	waitCtx, stopWaiting := context.WithTimeout(context.Background(), time.Second)
	defer stopWaiting()
	if err := app.seeder.WaitForStop(waitCtx); err != nil {
		t.Fatal(err)
	}
}

func TestAppStopReportsPriorityRecoveryFailure(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	adapter.priorityRestoreError = errors.New("original priority backup cannot be read")
	if _, err := app.StopSeed(); !errors.Is(err, adapter.priorityRestoreError) {
		t.Fatalf("stop concealed failed priority restoration: %v", err)
	}
	if adapter.priorityRestores.Load() != 1 {
		t.Fatal("stop did not attempt recovery")
	}
}

func TestAppManualRestoreRecoversGraphicsEvenIfPriorityRecoveryFails(t *testing.T) {
	app, adapter, original := newTestApp(t)
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	adapter.priorityRestoreError = errors.New("priority backup is unreadable")
	ok, err := app.RestoreSquadConfig()
	if ok || !errors.Is(err, adapter.priorityRestoreError) {
		t.Fatalf("partial recovery reported success: %t %v", ok, err)
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("independent verified graphics recovery was skipped: %v", err)
	}
}

func TestAppPriorityFailureDoesNotAbortSeedingAndPreservesBackup(t *testing.T) {
	app, adapter, original := newTestApp(t)
	settings := app.cfgMgr.Get()
	settings.Potato.AutoRestoreOnExit = false
	if err := app.cfgMgr.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.seeder = seed.NewEngine(adapter, app.gameCtrl, testPendingFleet{}, app.cfgMgr, nil)
	server := citadel.Server{ID: "1", Status: "online", CurrentPlayers: 10, MaxPlayers: 100}
	plan := seed.Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: seed.PostSeedDoNothing}
	if err := app.seeder.Start(ctx, plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}

	// 1. Simulate an arbitrary platform failure (distinct from Access Denied, e.g. snapshot failure)
	adapter.isSquadRunning.Store(true)
	adapter.priorityError = errors.New("kernel snapshot error or unexpected Win32 failure")

	// 2. Applying priority should NOT fail the run or return an error to engine
	isApplied, isCurrent, err := app.applySquadPriorityForRun(app.seeder.CurrentRunID())
	if err != nil {
		t.Fatalf("unexpected error returned from applySquadPriorityForRun: %v", err)
	}
	if !isCurrent {
		t.Fatal("expected run to remain current")
	}
	if !isApplied {
		t.Fatal("expected isApplied to be true to halt polling after platform error")
	}

	// 3. Crucial assertion: the seeding mission must REMAIN ACTIVE!
	session := app.seeder.GetSession()
	if !session.State.IsActive() {
		t.Fatalf("priority failure aborted active seed mission; state=%s, error=%s", session.State, session.ErrorMessage)
	}

	// 4. Clean shutdown works normally
	if err := app.seeder.Stop(); err != nil {
		t.Fatal(err)
	}
	waitCtx, stopWaiting := context.WithTimeout(context.Background(), time.Second)
	defer stopWaiting()
	if err := app.seeder.WaitForStop(waitCtx); err != nil {
		t.Fatal(err)
	}
	if !app.squadConfig.HasActiveBackup() {
		t.Fatal("priority failure discarded the original settings backup")
	}
	adapter.isSquadRunning.Store(false)
	ok, err := app.RestoreSquadConfig()
	if !ok || err != nil {
		t.Fatalf("manual squad restore failed: ok=%v, err=%v", ok, err)
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("settings were not restored cleanly: %v", err)
	}
}

type testUnavailableFleet struct{}

func (testUnavailableFleet) GetServers(context.Context) ([]citadel.Server, error) {
	return nil, errors.New("live fleet is unavailable")
}
func (testUnavailableFleet) ResolveJoinURL(context.Context, string) (string, error) {
	return "", errors.New("target connection is unavailable")
}

func newTestApp(t *testing.T) (*App, *testAppPlatform, []byte) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "app-config"))
	t.Setenv("APPDATA", filepath.Join(root, "app-config"))
	settings, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "GameUserSettings.ini")
	original := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=144.000000\r\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := &testAppPlatform{}
	controller := game.NewController(adapter)
	app := &App{
		cfgMgr: settings, platform: adapter, gameCtrl: controller,
		citadel:     citadel.NewClient(settings.Get().WebApiBaseUrl),
		squadConfig: game.NewConfigManagerWithPaths(path, filepath.Join(root, "backup")),
		seeder:      seed.NewEngine(adapter, controller, testUnavailableFleet{}, settings, nil),
	}
	return app, adapter, original
}

func TestAppCancellationDoesNotAbortUnownedSystemShutdown(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	if err := app.CancelPowerAction(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.StopSeed(); err != nil {
		t.Fatal(err)
	}
	if adapter.shutdownCancels.Load() != 0 {
		t.Fatal("Sentry attempted to cancel a system shutdown it did not schedule")
	}
}

func TestAppDoesNotRestoreWhenProcessInspectionFails(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	adapter.processError = errors.New("process inspection is unavailable")
	if _, err := app.RestoreSquadConfig(); err == nil {
		t.Fatal("unknown process state was treated as a confirmed stopped game")
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, patched) || !app.squadConfig.HasActiveBackup() {
		t.Fatalf("unknown game state allowed config mutation: %v", err)
	}
}

func TestSystemStatusDoesNotClaimUnknownProcessesAreStopped(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	adapter.processError = errors.New("process inspection is unavailable")
	status := app.GetSystemStatus()
	if status["squadRunning"] != nil || status["steamRunning"] != nil {
		t.Fatalf("unknown process status was reported as confirmed: %v", status)
	}
	message, ok := status["processStatusError"].(string)
	if !ok || !strings.Contains(message, adapter.processError.Error()) {
		t.Fatalf("process inspection error was hidden: %v", status)
	}
}

func TestAppInitialBackupRequiresVerifiedStoppedGame(t *testing.T) {
	app, adapter, original := newTestApp(t)
	adapter.processError = errors.New("cannot inspect Squad")
	if err := app.backupInitialSquadConfig(); err == nil {
		t.Fatal("initial backup ignored process inspection failure")
	}
	if app.squadConfig.GetStatus().HasBackup {
		t.Fatal("unverified initial state became the original backup")
	}
	adapter.processError = nil
	if err := app.backupInitialSquadConfig(); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(app.squadConfig.BackupPath())
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("confirmed initial backup differs from original settings: %v", err)
	}
}

func TestInterruptedGraphicsRecoveryDoesNotDependOnReadableAppSettings(t *testing.T) {
	for _, hasUnknownProcessState := range []bool{false, true} {
		t.Run(map[bool]string{false: "verified exit", true: "unknown game state"}[hasUnknownProcessState], func(t *testing.T) {
			app, adapter, original := newTestApp(t)
			if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
				t.Fatal(err)
			}
			root, err := config.StateDirectory()
			if err != nil {
				t.Fatal(err)
			}
			invalidSettings := []byte(`{"potato":null}`)
			settingsPath := filepath.Join(root, "config.json")
			if err := os.WriteFile(settingsPath, invalidSettings, 0o600); err != nil {
				t.Fatal(err)
			}
			if hasUnknownProcessState {
				adapter.processError = errors.New("process snapshot failed")
			}
			err = restoreInterruptedSquadConfig(adapter, app.squadConfig)
			if hasUnknownProcessState {
				if err == nil || !app.squadConfig.HasActiveBackup() {
					t.Fatal("unknown game state discarded recovery")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				current, err := os.ReadFile(app.squadConfig.ConfigPath())
				if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
					t.Fatalf("invalid app preferences blocked verified graphics recovery: %v", err)
				}
			}
			data, err := os.ReadFile(settingsPath)
			if err != nil || !bytes.Equal(data, invalidSettings) {
				t.Fatalf("recovery invented replacement application preferences: %v", err)
			}
		})
	}
}

func TestAppSaveSettingsUsesPersistedEndpoint(t *testing.T) {
	app, _, _ := newTestApp(t)
	updated := app.cfgMgr.Get()
	updated.WebApiBaseUrl = "https://citadel.example.com"
	if !app.SaveSettings(updated) {
		t.Fatal("settings were not saved")
	}
	if app.citadel.GetBaseURL() != app.cfgMgr.Get().WebApiBaseUrl {
		t.Fatal("runtime endpoint differs from the settings actually persisted")
	}
}

func TestAppRestoreWaitsForLaunchOperationAndRechecksGameState(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	app.operationMu.Lock()
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		_, err := app.RestoreSquadConfig()
		result <- err
	}()
	<-started
	select {
	case err := <-result:
		app.operationMu.Unlock()
		t.Fatalf("restoration bypassed the launch operation: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	adapter.isSquadRunning.Store(true)
	app.operationMu.Unlock()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("restoration did not recheck the newly running game")
		}
	case <-time.After(time.Second):
		t.Fatal("serialized restoration did not return")
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, patched) {
		t.Fatalf("running game's settings were changed: %v", err)
	}
}

func TestAppFailedMissionRollsBackWithAutomaticExitRestoreDisabled(t *testing.T) {
	app, _, original := newTestApp(t)
	settings := app.cfgMgr.Get()
	settings.Potato.AutoRestoreOnExit = false
	if err := app.cfgMgr.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	server := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10}
	plan := seed.Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: seed.PostSeedDoNothing}
	if err := app.seeder.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for app.seeder.HasRunningWorker() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if app.seeder.HasRunningWorker() || app.seeder.GetSession().State != seed.StateFailed {
		t.Fatal("test mission did not finish with the verified connection failure")
	}
	if err := app.restoreSquadAutomaticallyIfStopped(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("failed mission did not restore the verified original: %v", err)
	}
}

func TestAppInterruptedPreparationKeepsRecoveryPendingAcrossProcessInspectionFailure(t *testing.T) {
	app, adapter, original := newTestApp(t)
	settings := app.cfgMgr.Get()
	settings.Potato.AutoRestoreOnExit = false
	if err := app.cfgMgr.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	adapter.processError = errors.New("cannot inspect Squad")
	if err := app.restoreSquadAutomaticallyIfStopped(); err == nil {
		t.Fatal("interrupted preparation did not report its pending recovery check")
	}
	if !app.squadConfig.HasActiveBackup() {
		t.Fatal("unknown process state discarded recovery")
	}
	adapter.processError = nil
	if err := app.restoreSquadAutomaticallyIfStopped(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("interrupted preparation was not restored: %v", err)
	}
}

type testPendingFleet struct{}

func (testPendingFleet) GetServers(ctx context.Context) ([]citadel.Server, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (testPendingFleet) ResolveJoinURL(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

type testReachedFleet struct{ server citadel.Server }

func (f testReachedFleet) GetServers(context.Context) ([]citadel.Server, error) {
	return []citadel.Server{f.server}, nil
}

func (testReachedFleet) ResolveJoinURL(context.Context, string) (string, error) {
	return "", errors.New("a reached target must not launch a game")
}

type testObservedFleet struct{ server citadel.Server }

func (f testObservedFleet) GetServers(context.Context) ([]citadel.Server, error) {
	return []citadel.Server{f.server}, nil
}
func (testObservedFleet) ResolveJoinURL(context.Context, string) (string, error) {
	return "steam://joinlobby/393380/1", nil
}

func TestAppOnlyTheOwningFinishedSessionCanKeepPotatoSettings(t *testing.T) {
	app, adapter, original := newTestApp(t)
	settings := app.cfgMgr.Get()
	settings.Potato.AutoRestoreOnExit = false
	if err := app.cfgMgr.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	server := citadel.Server{ID: "1", Status: "online", CurrentPlayers: 10, MaxPlayers: 100, ServerIP: "192.0.2.1", GamePort: 7812}
	adapter.canLaunch = true
	adapter.isSquadRunning.Store(true)
	app.seeder = seed.NewEngine(adapter, app.gameCtrl, testObservedFleet{server: server}, app.cfgMgr, nil)
	plan := seed.Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: seed.PostSeedDoNothing}
	if err := app.seeder.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	app.potatoRunID = app.seeder.CurrentRunID()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	deadline := time.Now().Add(time.Second)
	for !app.seeder.GetSession().HasObservedGame && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !app.seeder.GetSession().HasObservedGame {
		t.Fatal("fixture never observed its simulated game")
	}
	if err := app.seeder.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := app.seeder.WaitForStop(ctx); err != nil {
		t.Fatal(err)
	}
	adapter.isSquadRunning.Store(false)
	if err := app.restoreSquadAutomaticallyIfStopped(); err != nil {
		t.Fatal(err)
	}
	if !app.squadConfig.HasActiveBackup() {
		t.Fatal("normal finished session ignored automatic restoration opt-out")
	}
	if err := app.squadConfig.RestoreOriginalConfig(); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	// No worker owns this new transaction; the old completed session must not authorize retention.
	if err := app.restoreSquadAutomaticallyIfStopped(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("old completion kept a new interrupted preparation: %v", err)
	}
}

func TestAppPreflightFullMissionRestoresPreparationWithoutAnObservedGame(t *testing.T) {
	app, adapter, original := newTestApp(t)
	settings := app.cfgMgr.Get()
	settings.Potato.AutoRestoreOnExit = false
	if err := app.cfgMgr.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	server := citadel.Server{ID: "1", Status: "full", CurrentPlayers: 100, MaxPlayers: 100}
	app.seeder = seed.NewEngine(adapter, app.gameCtrl, testReachedFleet{server: server}, app.cfgMgr, nil)
	plan := seed.Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: seed.PostSeedDoNothing}
	if err := app.seeder.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	app.potatoRunID = app.seeder.CurrentRunID()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.seeder.WaitForStop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := app.restoreSquadAutomaticallyIfStopped(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("preflight-only completion kept settings from an unstarted game: %v", err)
	}
}

func TestAppShutdownWaitsForOwnedSleepResourceAndRestoresAfterGameExit(t *testing.T) {
	app, adapter, original := newTestApp(t)
	settings := app.cfgMgr.Get()
	settings.Power.PreventSleepDuringSeed = true
	settings.Potato.AutoRestoreOnExit = true
	if err := app.cfgMgr.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.squadConfig.ApplyPotatoConfig(1, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	app.backgroundCancel = cancel
	app.seeder = seed.NewEngine(adapter, app.gameCtrl, testPendingFleet{}, app.cfgMgr, nil)
	server := citadel.Server{ID: "1", Status: "online", CurrentPlayers: 10, MaxPlayers: 100}
	plan := seed.Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: seed.PostSeedDoNothing}
	if err := app.seeder.Start(ctx, plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for adapter.sleepAcquires.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if adapter.sleepAcquires.Load() != 1 {
		t.Fatal("test worker did not acquire its simulated sleep resource")
	}
	app.shutdown(context.Background())
	app.shutdown(context.Background())
	if app.seeder.HasRunningWorker() || adapter.sleepReleases.Load() != 1 {
		t.Fatal("application exited without completing worker/resource cleanup")
	}
	current, err := os.ReadFile(app.squadConfig.ConfigPath())
	if err != nil || !bytes.Equal(current, original) || app.squadConfig.HasActiveBackup() {
		t.Fatalf("shutdown did not restore the verified original: %v", err)
	}
	if _, err := app.StartSeed(plan); err == nil {
		t.Fatal("seeding was allowed after shutdown started")
	}
	if _, err := app.JoinServerDirect(server.ID); err == nil {
		t.Fatal("direct joining was allowed after shutdown started")
	}
}

func TestJoinServerDirectSurfacesInGameInviteRejection(t *testing.T) {
	app, adapter, _ := newTestApp(t)
	adapter.isSquadRunning.Store(true)

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "SquadGame.log")
	if err := os.WriteFile(logPath, []byte("LogInit: Display: Running Squad\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app.gameCtrl.SetLogPaths(func() []string { return []string{logPath} })

	adapter.onLaunch = func(uri string) error {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("LogOnlineSession: Warning: STEAM: Removed incompatible build: ServerBuildUniqueId = 0x000a7f2c, GetBuildUniqueId() = 0x000a9aa0\n" +
			"LogRedpointEOSPlatformIntegrationOSSv1: Error: Unable to find the 'RedpointEOSRoomId' attribute on the session invite\n")
		return err
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/servers/sentry" {
			_, _ = w.Write([]byte(`[{"id":"1","name":"[RU] CITADEL [AAS/RAAS]","players":10,"maxPlayers":100,"queue":0,"status":"online"}]`))
			return
		}
		if r.URL.Path == "/api/servers/1/connect" {
			_, _ = w.Write([]byte(`{"status":"ready","method":"steam_joinlobby","joinUrl":"steam://joinlobby/393380/109775242238387378","steamAppId":"393380"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()
	app.citadel.SetBaseURL(mockServer.URL)

	success, err := app.JoinServerDirect("1")
	if success {
		t.Fatal("JoinServerDirect returned success for an incompatible lobby invitation")
	}
	if err == nil || !strings.Contains(err.Error(), "687916") || !strings.Contains(err.Error(), "694944") {
		t.Fatalf("JoinServerDirect failed to surface build mismatch: %v", err)
	}
}

func TestAppBaselineAndRestorePoints(t *testing.T) {
	app, _, original := newTestApp(t)

	// Baseline should exist upon init
	if !app.HasInitialBaseline() {
		t.Fatal("expected app to report initial baseline exists")
	}

	gpu := app.GetGPUInfo()
	if gpu.Model == "" {
		t.Fatalf("expected non-empty GPU model, got %+v", gpu)
	}

	// Create a restore point
	rp, err := app.CreateRestorePoint("Test Point", "Description")
	if err != nil {
		t.Fatalf("CreateRestorePoint failed: %v", err)
	}
	if rp.ID == "" || rp.Name != "Test Point" {
		t.Fatalf("unexpected restore point: %+v", rp)
	}

	// List points
	points, err := app.ListRestorePoints()
	if err != nil {
		t.Fatalf("ListRestorePoints failed: %v", err)
	}
	if len(points) != 1 || points[0].ID != rp.ID {
		t.Fatalf("expected 1 restore point, got %+v", points)
	}

	// Modify config
	modified := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=60.000000\r\n")
	if err := os.WriteFile(app.squadConfig.ConfigPath(), modified, 0o600); err != nil {
		t.Fatal(err)
	}

	// Restore snapshot
	ok, err := app.RestoreRestorePoint(rp.ID)
	if err != nil || !ok {
		t.Fatalf("RestoreRestorePoint failed: %v", err)
	}
	data, _ := os.ReadFile(app.squadConfig.ConfigPath())
	if !bytes.Equal(data, original) {
		t.Fatalf("restored data != original, got: %s", data)
	}

	// Delete snapshot
	ok, err = app.DeleteRestorePoint(rp.ID)
	if err != nil || !ok {
		t.Fatalf("DeleteRestorePoint failed: %v", err)
	}
	points, _ = app.ListRestorePoints()
	if len(points) != 0 {
		t.Fatalf("expected 0 restore points after delete, got %d", len(points))
	}

	// Modify config again
	if err := os.WriteFile(app.squadConfig.ConfigPath(), modified, 0o600); err != nil {
		t.Fatal(err)
	}

	// Restore initial baseline
	ok, err = app.RestoreInitialBaseline()
	if err != nil || !ok {
		t.Fatalf("RestoreInitialBaseline failed: %v", err)
	}
	data, _ = os.ReadFile(app.squadConfig.ConfigPath())
	if !bytes.Equal(data, original) {
		t.Fatalf("baseline restored data != original, got: %s", data)
	}
}

func TestTelemetryAutoDispatchesOnTerminalState(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	app, _, _ := newTestApp(t)
	app.telemetryReporter = telemetry.NewReporter(true, server.URL, "1.3.3")

	// Trigger failed terminal state
	app.handleSessionStateChange(seed.Session{
		State:        seed.StateFailed,
		ErrorMessage: "test simulated failure",
	})

	// Wait for async dispatch
	app.telemetryWg.Wait()

	if hits.Load() != 1 {
		t.Fatalf("expected 1 telemetry dispatch on terminal state, got %d", hits.Load())
	}
}

func TestTelemetryDeduplicationOnSameRun(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	app, _, _ := newTestApp(t)
	app.telemetryReporter = telemetry.NewReporter(true, server.URL, "1.3.3")

	// Trigger multiple terminal state callbacks
	app.handleSessionStateChange(seed.Session{
		State:        seed.StateStopped,
		ErrorMessage: "first stop",
	})
	app.handleSessionStateChange(seed.Session{
		State:        seed.StateStopped,
		ErrorMessage: "second stop duplicate",
	})

	app.telemetryWg.Wait()

	if hits.Load() != 1 {
		t.Fatalf("expected exactly 1 dispatch despite multiple terminal events, got %d", hits.Load())
	}
}
