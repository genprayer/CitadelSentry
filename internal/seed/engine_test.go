package seed

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/config"
	"citadel-seeder/internal/platform"
)

func TestStoppedRunRejectsLateProgress(t *testing.T) {
	engine := NewEngine(fakeSystemController{}, &fakeGameController{}, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
	engine.runID = 1
	engine.done = make(chan struct{})
	engine.session.State = StateActive
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	if engine.update(engine.runID, func(session *Session) { session.State = StateActive }) {
		t.Fatal("late worker progress resumed a session after Stop")
	}
	if engine.GetSession().State != StateStopped {
		t.Fatal("stopped state was overwritten")
	}
}

func TestStopAtCompletionPreventsPostSeedClose(t *testing.T) {
	t.Parallel()
	server := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10, TargetSeedCount: 50}
	game := &fakeGameController{connected: make(chan struct{}, 1)}
	provider := &fakeServerProvider{servers: []citadel.Server{server}, targetAt: map[string]int{server.ID: 2}}
	var engine *Engine
	engine = NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateCompleted {
			_ = engine.Stop()
		}
	})
	engine.progressEvery = time.Millisecond
	engine.statusEvery = time.Millisecond
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedCloseSquad}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	game.mu.Lock()
	defer game.mu.Unlock()
	if game.closeCount != 0 {
		t.Fatal("post-seed close executed after the user stopped the session")
	}
}

func TestBuildQueueRejectsUnknownServerStatus(t *testing.T) {
	for _, status := range []string{"", "unknown", "unexpected-status"} {
		server := citadel.Server{ID: "1", Name: "Unverified", Status: status}
		if _, err := BuildQueue(Plan{ServerID: server.ID}, []citadel.Server{server}); err == nil {
			t.Errorf("unverified server status %q was queued", status)
		}
	}
}

func TestEngineConcurrentStartsKeepOneWorker(t *testing.T) {
	t.Parallel()
	server := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10}
	game := &fakeGameController{connected: make(chan struct{}, 16)}
	engine := NewEngine(fakeSystemController{}, game, &fakeServerProvider{servers: []citadel.Server{server}}, fakeSettingsProvider{}, nil)
	t.Cleanup(func() {
		if err := engine.Stop(); err != nil {
			t.Errorf("stop test worker: %v", err)
		}
		waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	})
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing}
	start := make(chan struct{})
	results := make(chan error, 16)
	for range cap(results) {
		go func() {
			<-start
			results <- engine.Start(context.Background(), plan, []citadel.Server{server})
		}()
	}
	close(start)
	successes := 0
	for range cap(results) {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent starts created %d workers, want one", successes)
	}
}

type unavailableServerProvider struct{}

func (unavailableServerProvider) GetServers(context.Context) ([]citadel.Server, error) {
	return nil, errors.New("live fleet could not be verified")
}

func (unavailableServerProvider) ResolveJoinURL(context.Context, string) (string, error) {
	return "steam://joinlobby/393380/1", nil
}

func TestEngineDoesNotLaunchWithUnverifiedLiveTelemetry(t *testing.T) {
	t.Parallel()
	server := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10}
	game := &fakeGameController{connected: make(chan struct{}, 1)}
	engine := NewEngine(fakeSystemController{}, game, unavailableServerProvider{}, fakeSettingsProvider{}, nil)
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return engine.GetSession().State == StateFailed || len(game.connected) > 0 })
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	if len(game.connected) > 0 {
		t.Fatal("failed live lookup was replaced with stale queue telemetry")
	}
}

func TestEngineDoesNotPowerOffWithoutObservedSquadProcess(t *testing.T) {
	t.Parallel()
	server := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10, TargetSeedCount: 50}
	game := &fakeGameController{connected: make(chan struct{}, 1), squadNotRun: true}
	provider := &fakeServerProvider{servers: []citadel.Server{server}, targetAt: map[string]int{server.ID: 2}}
	var mu sync.Mutex
	var actions []string
	engine := NewEngine(fakeSystemController{mu: &mu, powerActions: &actions}, game, provider, fakeSettingsProvider{}, nil)
	engine.connectionTimeout = 50 * time.Millisecond
	engine.durationUnit = 100 * time.Millisecond
	engine.progressEvery = time.Millisecond
	engine.statusEvery = 20 * time.Millisecond
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedShutdown}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 0 {
		t.Fatalf("URI dispatch without a Squad process caused power actions: %v", actions)
	}
}

type lateObservedGame struct {
	fakeGameController
	checks int
}

func (g *lateObservedGame) CheckSquadRunning() (bool, error) {
	g.checks++
	return g.checks > 1, nil
}

func (g *lateObservedGame) CheckServerConnection(citadel.Server) (bool, error) {
	if g.checks == 2 {
		time.Sleep(40 * time.Millisecond)
	}
	return true, nil
}

func TestEngineDoesNotCountLaunchWaitAsObservedGameDuration(t *testing.T) {
	server := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10, TargetSeedCount: 50}
	game := &lateObservedGame{}
	provider := &fakeServerProvider{servers: []citadel.Server{server}, targetAt: map[string]int{server.ID: 2}}
	var mu sync.Mutex
	var actions []string
	engine := NewEngine(fakeSystemController{mu: &mu, powerActions: &actions}, game, provider, fakeSettingsProvider{}, nil)
	engine.durationUnit = time.Second
	engine.progressEvery = 2 * time.Millisecond
	engine.statusEvery = 40 * time.Millisecond
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedShutdown}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	if !engine.GetSession().HasObservedGame {
		t.Fatal("fixture never observed a game process")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 0 {
		t.Fatalf("waiting for launch was counted as active game time: %v", actions)
	}
	game.mu.Lock()
	defer game.mu.Unlock()
	if game.closeCount != 0 {
		t.Fatal("short observed session closed Squad for a rejected power action")
	}
}

func TestEngineCancelAtCountdownCommitPreventsPowerAction(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var actions []string
	engine := NewEngine(fakeSystemController{mu: &mu, powerActions: &actions}, &fakeGameController{squadNotRun: true}, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
	engine.durationUnit = time.Millisecond
	engine.SetPowerCountdownCallback(func(_ string, remaining int) {
		if remaining == 0 {
			if err := engine.CancelPowerAction(); err != nil {
				t.Errorf("cancel countdown: %v", err)
			}
		}
	})
	if err := engine.performPostSeedAction(context.Background(), PostSeedShutdown, true, time.Second, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, action := range actions {
		if action == PostSeedShutdown {
			t.Fatal("shutdown executed after countdown cancellation")
		}
	}
}

type fakeServerProvider struct {
	mu       sync.Mutex
	servers  []citadel.Server
	polls    int
	targetAt map[string]int
}

func (f *fakeServerProvider) GetServers(context.Context) ([]citadel.Server, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++

	result := append([]citadel.Server(nil), f.servers...)
	for index := range result {
		if targetPoll := f.targetAt[result[index].ID]; targetPoll > 0 && f.polls >= targetPoll {
			result[index].CurrentPlayers = result[index].TargetSeedCount
		}
	}
	return result, nil
}

func (f *fakeServerProvider) ResolveJoinURL(_ context.Context, serverID string) (string, error) {
	return fmt.Sprintf("steam://joinlobby/393380/%s/76561198000000000", serverID), nil
}

type fakeGameController struct {
	mu              sync.Mutex
	connects        []string
	closeCount      int
	switchCount     int
	connected       chan struct{}
	squadNotRun     bool
	processError    error
	closeError      error
	exitError       error
	noConfirmation  bool
	connectionError error
}

func (f *fakeGameController) ConnectResolved(ctx context.Context, resolve func(context.Context) (string, error), shouldSwitch bool) error {
	uri, err := resolve(ctx)
	if err != nil {
		return err
	}
	if shouldSwitch {
		return f.SwitchServer(ctx, uri)
	}
	return f.Connect(ctx, uri)
}

func (f *fakeGameController) CheckServerConnection(citadel.Server) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.squadNotRun && !f.noConfirmation, f.connectionError
}

func (f *fakeGameController) GetObservedConnectedServerID() string {
	return ""
}

func (f *fakeGameController) Connect(ctx context.Context, joinURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	f.connects = append(f.connects, joinURL)
	f.mu.Unlock()
	select {
	case f.connected <- struct{}{}:
	default:
	}
	return nil
}

func (f *fakeGameController) CloseSquad() error {
	f.mu.Lock()
	f.closeCount++
	f.mu.Unlock()
	return f.closeError
}

func (f *fakeGameController) SwitchServer(ctx context.Context, joinURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	f.switchCount++
	f.closeCount++
	f.mu.Unlock()
	return f.Connect(ctx, joinURL)
}

func (f *fakeGameController) IsSquadRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.squadNotRun
}

func (f *fakeGameController) CheckSquadRunning() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.squadNotRun, f.processError
}

func (f *fakeGameController) WaitForSquadExit(ctx context.Context, _ time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.exitError
}

func TestPowerActionStopsWhenGameCannotBeClosed(t *testing.T) {
	for _, test := range []struct {
		name       string
		closeError error
		exitError  error
	}{
		{name: "close command failed", closeError: errors.New("close command failed")},
		{name: "game stayed running", exitError: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			var actions []string
			game := &fakeGameController{closeError: test.closeError, exitError: test.exitError}
			engine := NewEngine(fakeSystemController{mu: &mu, powerActions: &actions}, game, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
			engine.durationUnit = time.Millisecond
			if err := engine.performPostSeedAction(context.Background(), PostSeedShutdown, true, time.Second, false); err == nil {
				t.Fatal("failed game shutdown was ignored")
			}
			mu.Lock()
			defer mu.Unlock()
			if len(actions) != 0 {
				t.Fatalf("power action executed without confirming game exit: %v", actions)
			}
		})
	}
}

func TestPowerActionRequiresConfirmedGameState(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	game := &fakeGameController{processError: errors.New("process inspection failed")}
	engine := NewEngine(fakeSystemController{mu: &mu, powerActions: &actions}, game, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
	engine.durationUnit = time.Millisecond
	if err := engine.performPostSeedAction(context.Background(), PostSeedShutdown, true, time.Second, false); err == nil {
		t.Fatal("unknown game state allowed a power action")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 0 || game.closeCount != 0 {
		t.Fatalf("native mutation attempted after inspection failure: actions=%v, closeCount=%d", actions, game.closeCount)
	}
}

type fakeSystemController struct {
	mu           *sync.Mutex
	powerActions *[]string
}

func (fakeSystemController) PreventSleep(context.Context, string) (platform.ReleaseFunc, error) {
	return func() {}, nil
}

func (f fakeSystemController) ExecutePowerAction(action string) error {
	if f.mu != nil && f.powerActions != nil {
		f.mu.Lock()
		defer f.mu.Unlock()
		*f.powerActions = append(*f.powerActions, action)
	}
	return nil
}

func (f fakeSystemController) CancelPowerAction() error {
	if f.mu != nil && f.powerActions != nil {
		f.mu.Lock()
		defer f.mu.Unlock()
		*f.powerActions = append(*f.powerActions, "cancel")
	}
	return nil
}

type fakeSettingsProvider struct{}

func TestEngineRejectsDurationOverflowBeforeStartingWorker(t *testing.T) {
	game := &fakeGameController{connected: make(chan struct{}, 1)}
	engine := NewEngine(fakeSystemController{}, game, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
	server := citadel.Server{ID: "1", Status: "online", MaxPlayers: 100}
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: config.MaxSeedDurationMinutes + 1, PostSeedAction: PostSeedDoNothing}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err == nil {
		t.Fatal("duration overflow started an immediate timeout workflow")
	}
	if engine.HasRunningWorker() || len(game.connected) != 0 || engine.GetSession().State != StateIdle {
		t.Fatal("invalid duration changed the idle state")
	}
}

func TestEngineRejectsUnknownCapacityBeforeStartingWorker(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			game := &fakeGameController{connected: make(chan struct{}, 1)}
			engine := NewEngine(fakeSystemController{}, game, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
			server := citadel.Server{ID: "1", Name: "Unknown capacity", Status: "online", MaxPlayers: capacity}
			plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing}
			if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err == nil {
				t.Fatal("unknown capacity started a worker")
			}
			if engine.HasRunningWorker() || len(game.connected) != 0 || engine.GetSession().State != StateIdle {
				t.Fatal("unknown capacity changed the idle state")
			}
		})
	}
}

func TestEngineRecordsConfirmedPreflightPlayerBaseline(t *testing.T) {
	queued := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100, CurrentPlayers: 10}
	live := queued
	live.CurrentPlayers = 20
	game := &fakeGameController{connected: make(chan struct{}, 1)}
	engine := NewEngine(fakeSystemController{}, game, &fakeServerProvider{servers: []citadel.Server{live}}, fakeSettingsProvider{}, nil)
	plan := Plan{ServerID: queued.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing}
	if err := engine.Start(context.Background(), plan, []citadel.Server{queued}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Stop(); err != nil {
			t.Error(err)
		}
		waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	})
	select {
	case <-game.connected:
	case <-time.After(time.Second):
		t.Fatal("fixture did not reach game dispatch")
	}
	if got := engine.GetSession().InitialPlayerCount; got != live.CurrentPlayers {
		t.Fatalf("initial count came from an older queue snapshot: %d", got)
	}
}

func TestEngineRejectsUnavailableLiveCapacityBeforeLaunching(t *testing.T) {
	for _, capacity := range []int{0, -1, 40} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			queued := citadel.Server{ID: "1", Name: "One", Status: "online", MaxPlayers: 100}
			live := queued
			live.MaxPlayers = capacity
			game := &fakeGameController{connected: make(chan struct{}, 1)}
			engine := NewEngine(fakeSystemController{}, game, &fakeServerProvider{servers: []citadel.Server{live}}, fakeSettingsProvider{}, nil)
			plan := Plan{ServerID: queued.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing}
			if err := engine.Start(context.Background(), plan, []citadel.Server{queued}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := engine.Stop(); err != nil {
					t.Error(err)
				}
				waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
			})
			select {
			case <-game.connected:
				t.Fatal("unavailable live capacity launched Squad")
			case <-time.After(50 * time.Millisecond):
			}
			waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
			if session := engine.GetSession(); session.State != StateFailed || session.ErrorMessage == "" {
				t.Fatalf("capacity failure was hidden: %+v", session)
			}
		})
	}
}

type sleepEnabledSettings struct{ fakeSettingsProvider }

func (s sleepEnabledSettings) Get() config.AppSettings {
	settings := s.fakeSettingsProvider.Get()
	settings.Power.PreventSleepDuringSeed = true
	return settings
}

type blockedCleanupSystem struct {
	fakeSystemController
	isReleasing chan struct{}
	canRelease  chan struct{}
}

func (s blockedCleanupSystem) PreventSleep(context.Context, string) (platform.ReleaseFunc, error) {
	return func() { close(s.isReleasing); <-s.canRelease }, nil
}

func TestStopPreservesFailureWhileWorkerReleasesResources(t *testing.T) {
	system := blockedCleanupSystem{isReleasing: make(chan struct{}), canRelease: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(system.canRelease) }) }
	t.Cleanup(release)
	engine := NewEngine(system, &fakeGameController{}, unavailableServerProvider{}, sleepEnabledSettings{}, nil)
	server := citadel.Server{ID: "1", Status: "online", CurrentPlayers: 10, MaxPlayers: 100}
	plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-system.isReleasing:
	case <-time.After(time.Second):
		t.Fatal("failed worker did not reach resource cleanup")
	}
	before := engine.GetSession()
	if before.State != StateFailed || before.ErrorMessage == "" || !engine.HasRunningWorker() {
		t.Fatal("fixture did not expose failure during live worker cleanup")
	}
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	after := engine.GetSession()
	if after.State != StateFailed || after.ErrorMessage != before.ErrorMessage {
		t.Fatal("Stop changed failure into a normal cancellation, allowing recovery opt-out")
	}
	release()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := engine.WaitForStop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRunOperationFailureAndConcurrentStopKeepFailureTerminal(t *testing.T) {
	engine := NewEngine(fakeSystemController{}, &fakeGameController{}, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
	engine.runID = 1
	engine.done = make(chan struct{})
	engine.session.State = StateActive
	entered := make(chan struct{})
	canFinish := make(chan struct{})
	operationDone := make(chan error, 1)
	go func() {
		isCurrent, err := engine.PerformRunOperation(1, func() error {
			close(entered)
			<-canFinish
			return errors.New("native setting could not be applied")
		})
		if !isCurrent {
			err = errors.New("active operation lost ownership")
		}
		operationDone <- err
	}()
	<-entered
	stopDone := make(chan error, 1)
	go func() { stopDone <- engine.Stop() }()
	close(canFinish)
	if err := <-operationDone; err == nil {
		t.Fatal("operation failure was hidden")
	}
	if err := <-stopDone; err != nil {
		t.Fatal(err)
	}
	if session := engine.GetSession(); session.State != StateFailed || session.ErrorMessage != "native setting could not be applied" {
		t.Fatalf("Stop overwrote failed native operation: %#v", session)
	}
	hasMutated := false
	isCurrent, err := engine.PerformRunOperation(1, func() error { hasMutated = true; return nil })
	if isCurrent || err != nil || hasMutated {
		t.Fatal("terminal worker allowed another native mutation")
	}
}

func (fakeSettingsProvider) Get() config.AppSettings {
	settings := config.DefaultSettings()
	settings.Power.PreventSleepDuringSeed = false
	return settings
}

func TestBuildQueueUsesSelectedOrderAndLimit(t *testing.T) {
	t.Parallel()

	fleet := []citadel.Server{
		{ID: "1", Name: "One", Status: "online"},
		{ID: "2", Name: "Two", Status: "offline"},
		{ID: "3", Name: "Three", Status: "online"},
	}

	queue, err := BuildQueue(Plan{ServerID: "3", ServerLimit: 2}, fleet)
	if err != nil {
		t.Fatalf("BuildQueue() error = %v", err)
	}
	if len(queue) != 2 || queue[0].ID != "3" || queue[1].ID != "1" {
		t.Fatalf("BuildQueue() = %#v, want IDs [3 1]", queue)
	}
}

func TestBuildQueueRejectsHiddenOrMissingExplicitServer(t *testing.T) {
	t.Parallel()

	fleet := []citadel.Server{{ID: "1", Name: "One", Status: "online"}}
	_, err := BuildQueue(Plan{ServerIDs: []string{"1", "2"}}, fleet)
	if err == nil {
		t.Fatal("BuildQueue() error = nil, want missing-server error")
	}
}

func TestEngineMovesThroughQueueAndRunsPostActionOnce(t *testing.T) {
	t.Parallel()

	servers := []citadel.Server{
		{ID: "1001", Name: "One", Status: "online", MaxPlayers: 100, TargetSeedCount: 50},
		{ID: "1002", Name: "Two", Status: "online", MaxPlayers: 100, TargetSeedCount: 50},
	}
	provider := &fakeServerProvider{
		servers:  servers,
		targetAt: map[string]int{"1001": 2, "1002": 4},
	}
	game := &fakeGameController{connected: make(chan struct{}, 4)}
	completed := make(chan Session, 1)
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateCompleted {
			select {
			case completed <- session:
			default:
			}
		}
	})
	engine.progressEvery = 5 * time.Millisecond
	engine.statusEvery = 5 * time.Millisecond

	plan := Plan{
		ServerID:           "1001",
		TargetPlayerCount:  50,
		MaxDurationMinutes: 1,
		ServerLimit:        2,
		PostSeedAction:     PostSeedCloseSquad,
		TimeoutAction:      TimeoutAbort,
	}
	if err := engine.Start(context.Background(), plan, servers); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case session := <-completed:
		if session.CurrentQueueIndex != 2 || session.TotalQueueCount != 2 {
			t.Fatalf("completed queue position = %d/%d, want 2/2", session.CurrentQueueIndex, session.TotalQueueCount)
		}
	case <-time.After(time.Second):
		t.Fatal("seed queue did not complete")
	}

	waitFor(t, time.Second, func() bool {
		game.mu.Lock()
		defer game.mu.Unlock()
		return len(game.connects) == 2 && game.switchCount == 1 && game.closeCount == 2
	})
}

func TestEngineStopCancelsActiveRun(t *testing.T) {
	t.Parallel()

	server := citadel.Server{
		ID: "2001", Name: "One", Status: "online", MaxPlayers: 100, TargetSeedCount: 50,
	}
	provider := &fakeServerProvider{servers: []citadel.Server{server}, targetAt: map[string]int{}}
	game := &fakeGameController{connected: make(chan struct{}, 1)}
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, nil)
	engine.progressEvery = 5 * time.Millisecond
	engine.statusEvery = time.Hour

	plan := Plan{
		ServerID:           server.ID,
		TargetPlayerCount:  50,
		MaxDurationMinutes: 1,
		ServerLimit:        1,
		PostSeedAction:     PostSeedCloseSquad,
		TimeoutAction:      TimeoutAbort,
	}
	if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-game.connected:
	case <-time.After(time.Second):
		t.Fatal("game was not connected")
	}
	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	waitFor(t, time.Second, func() bool {
		return engine.GetSession().State == StateStopped
	})
	game.mu.Lock()
	defer game.mu.Unlock()
	if game.closeCount != 0 {
		t.Fatalf("CloseSquad() count = %d, want 0 for manual stop", game.closeCount)
	}
}

func TestEngineContinuesToNextServerAfterTimeout(t *testing.T) {
	t.Parallel()

	servers := []citadel.Server{
		{ID: "3001", Name: "One", Status: "online", MaxPlayers: 100, TargetSeedCount: 50},
		{ID: "3002", Name: "Two", Status: "online", MaxPlayers: 100, TargetSeedCount: 50},
	}
	provider := &fakeServerProvider{servers: servers, targetAt: map[string]int{}}
	game := &fakeGameController{connected: make(chan struct{}, 4)}
	completed := make(chan Session, 1)
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateTimedOut {
			select {
			case completed <- session:
			default:
			}
		}
	})
	engine.durationUnit = 40 * time.Millisecond
	engine.progressEvery = time.Millisecond
	engine.statusEvery = time.Millisecond

	plan := Plan{
		ServerID:           "3001",
		TargetPlayerCount:  50,
		MaxDurationMinutes: 1,
		ServerLimit:        2,
		PostSeedAction:     PostSeedCloseSquad,
		TimeoutAction:      TimeoutContinueNext,
	}
	if err := engine.Start(context.Background(), plan, servers); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	for index := 0; index < 2; index++ {
		select {
		case <-game.connected:
		case <-time.After(time.Second):
			t.Fatal("seed queue did not connect to both servers")
		}
	}

	provider.mu.Lock()
	provider.servers[1].CurrentPlayers = 50
	provider.mu.Unlock()

	select {
	case session := <-completed:
		if session.CurrentQueueIndex != 2 || session.TotalQueueCount != 2 || session.SkippedServerCount != 1 {
			t.Fatalf("completed queue position = %d/%d, want 2/2", session.CurrentQueueIndex, session.TotalQueueCount)
		}
	case <-time.After(time.Second):
		t.Fatal("seed queue did not complete after first-server timeout")
	}

	game.mu.Lock()
	defer game.mu.Unlock()
	if game.closeCount != 1 {
		t.Fatalf("skipped queue executed successful-queue close: %d closes, want only the server switch", game.closeCount)
	}
	if game.switchCount != 1 {
		t.Fatalf("SwitchServer() count = %d, want 1", game.switchCount)
	}
}

func TestEngineTimeoutCloseSquadAction(t *testing.T) {
	t.Parallel()

	servers := []citadel.Server{
		{ID: "2001", Name: "One", Status: "online", MaxPlayers: 100, TargetSeedCount: 50, CurrentPlayers: 10},
	}
	provider := &fakeServerProvider{
		servers: servers,
	}
	game := &fakeGameController{connected: make(chan struct{}, 2)}
	timedOut := make(chan Session, 1)
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateTimedOut {
			select {
			case timedOut <- session:
			default:
			}
		}
	})
	engine.progressEvery = 5 * time.Millisecond
	engine.durationUnit = 30 * time.Millisecond
	engine.progressEvery = time.Millisecond
	engine.statusEvery = time.Millisecond

	plan := Plan{
		ServerID:           "2001",
		TargetPlayerCount:  50,
		MaxDurationMinutes: 1,
		ServerLimit:        1,
		TimeoutAction:      TimeoutCloseSquad,
	}
	if err := engine.Start(context.Background(), plan, servers); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-timedOut:
	case <-time.After(time.Second):
		t.Fatal("session did not time out")
	}
	// The terminal event precedes post-action cleanup; observe the worker before asserting it.
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := engine.WaitForStop(waitCtx); err != nil {
		t.Fatal(err)
	}

	game.mu.Lock()
	defer game.mu.Unlock()
	if game.closeCount < 1 {
		t.Fatalf("expected game.CloseSquad() to be called on TimeoutCloseSquad, closeCount = %d", game.closeCount)
	}
}

func TestEngineStopsWhenSquadProcessClosed(t *testing.T) {
	t.Parallel()

	servers := []citadel.Server{
		{ID: "3001", Name: "One", Status: "online", MaxPlayers: 100, TargetSeedCount: 50, CurrentPlayers: 10},
	}
	provider := &fakeServerProvider{
		servers: servers,
	}
	game := &fakeGameController{connected: make(chan struct{}, 2)}
	failed := make(chan Session, 1)
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateFailed {
			select {
			case failed <- session:
			default:
			}
		}
	})
	engine.durationUnit = 50 * time.Millisecond
	engine.progressEvery = time.Millisecond
	engine.statusEvery = time.Millisecond

	plan := Plan{
		ServerID:           "3001",
		TargetPlayerCount:  50,
		MaxDurationMinutes: 1,
		ServerLimit:        1,
		TimeoutAction:      TimeoutAbort,
	}
	if err := engine.Start(context.Background(), plan, servers); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait until game connected
	select {
	case <-game.connected:
	case <-time.After(time.Second):
		t.Fatal("game did not connect")
	}

	// Wait briefly so statusTicker can register squadStarted = true
	time.Sleep(5 * time.Millisecond)

	// Simulate user closing Squad process
	game.mu.Lock()
	game.squadNotRun = true
	game.mu.Unlock()

	select {
	case session := <-failed:
		if session.ErrorMessage != "squad process was closed" {
			t.Fatalf("session.ErrorMessage = %q, want %q", session.ErrorMessage, "squad process was closed")
		}
		if session.State.IsActive() {
			t.Fatal("session.State.IsActive() should be false after failure")
		}
	case <-time.After(time.Second):
		t.Fatal("engine did not fail after squad process was closed")
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func TestEngineAbortsSleepWhenPreflightAllTargetReached(t *testing.T) {
	t.Parallel()

	servers := []citadel.Server{
		{ID: "4001", Name: "FullServer", Status: "online", MaxPlayers: 100, TargetSeedCount: 50, CurrentPlayers: 60},
	}
	provider := &fakeServerProvider{servers: servers}
	game := &fakeGameController{connected: make(chan struct{}, 2)}
	completed := make(chan Session, 1)

	var mu sync.Mutex
	var powerActions []string
	sys := fakeSystemController{mu: &mu, powerActions: &powerActions}

	engine := NewEngine(sys, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateCompleted {
			select {
			case completed <- session:
			default:
			}
		}
	})
	engine.durationUnit = 50 * time.Millisecond
	engine.progressEvery = time.Millisecond
	engine.statusEvery = time.Millisecond

	plan := Plan{
		ServerID:           "4001",
		TargetPlayerCount:  50,
		MaxDurationMinutes: 1,
		ServerLimit:        1,
		PostSeedAction:     PostSeedSleep,
	}

	if err := engine.Start(context.Background(), plan, servers); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case session := <-completed:
		if session.State != StateCompleted {
			t.Fatalf("session state = %s, want %s", session.State, StateCompleted)
		}
	case <-time.After(time.Second):
		t.Fatal("session did not complete")
	}

	mu.Lock()
	actionsCount := len(powerActions)
	mu.Unlock()

	if actionsCount > 0 {
		t.Fatalf("powerActions = %#v, want none (aborted due to preflight full server)", powerActions)
	}
}

func TestEngineCancelPowerActionAbortsPendingCountdown(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var powerActions []string
	sys := fakeSystemController{mu: &mu, powerActions: &powerActions}
	game := &fakeGameController{}

	engine := NewEngine(sys, game, &fakeServerProvider{}, fakeSettingsProvider{}, nil)
	// Cancel when nothing pending returns nil safely
	if err := engine.CancelPowerAction(); err != nil {
		t.Fatalf("CancelPowerAction() error = %v", err)
	}

	mu.Lock()
	actions := append([]string(nil), powerActions...)
	mu.Unlock()

	if len(actions) != 0 {
		t.Fatalf("no Sentry power action was scheduled; system cancellation must not run: %v", actions)
	}
}

func TestEngineSkipCurrentServer(t *testing.T) {
	t.Parallel()

	servers := []citadel.Server{
		{ID: "5001", Name: "ServerOne", Status: "online", MaxPlayers: 100, TargetSeedCount: 50},
		{ID: "5002", Name: "ServerTwo", Status: "online", MaxPlayers: 100, TargetSeedCount: 50},
	}
	provider := &fakeServerProvider{
		servers:  servers,
		targetAt: map[string]int{"5002": 2},
	}
	game := &fakeGameController{connected: make(chan struct{}, 4)}
	completed := make(chan Session, 1)
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, func(session Session) {
		if session.State == StateCompleted {
			select {
			case completed <- session:
			default:
			}
		}
	})
	engine.progressEvery = 5 * time.Millisecond
	engine.statusEvery = 5 * time.Millisecond

	plan := Plan{
		ServerID:           "5001",
		TargetPlayerCount:  50,
		MaxDurationMinutes: 10,
		ServerLimit:        2,
		TimeoutAction:      TimeoutAbort,
	}
	if err := engine.Start(context.Background(), plan, servers); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait until connected to server 1
	select {
	case <-game.connected:
	case <-time.After(time.Second):
		t.Fatal("game did not connect to first server")
	}

	// Skip the first server!
	if err := engine.SkipCurrentServer(); err != nil {
		t.Fatalf("SkipCurrentServer() error = %v", err)
	}

	// It should now advance to ServerTwo and complete when targetAt is reached
	select {
	case session := <-completed:
		if session.SkippedServerCount != 1 {
			t.Fatalf("session.SkippedServerCount = %d, want 1", session.SkippedServerCount)
		}
		if session.CurrentQueueIndex != 2 {
			t.Fatalf("session.CurrentQueueIndex = %d, want 2", session.CurrentQueueIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("engine did not advance and complete after skip")
	}
}

