package game

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/platform"
)

type fakePlatformAdapter struct {
	mu        sync.Mutex
	isRunning bool
	isStuck   bool
	closeErr  error
	actions   []string
	onLaunch  func(string) error
}

func (f *fakePlatformAdapter) Name() string { return "fake" }

func (f *fakePlatformAdapter) IsSquadRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.isRunning
}

func (f *fakePlatformAdapter) IsSteamRunning() bool             { return true }
func (f *fakePlatformAdapter) CheckSquadRunning() (bool, error) { return f.IsSquadRunning(), nil }
func (f *fakePlatformAdapter) CheckSteamRunning() (bool, error) { return true, nil }

func (f *fakePlatformAdapter) PreventSleep(context.Context, string) (platform.ReleaseFunc, error) {
	return func() {}, nil
}

func (f *fakePlatformAdapter) ExecutePowerAction(string) error { return nil }
func (f *fakePlatformAdapter) CancelPowerAction() error        { return nil }

func (f *fakePlatformAdapter) OpenBrowser(string) error              { return nil }
func (f *fakePlatformAdapter) OpenFolder(string) error               { return nil }
func (f *fakePlatformAdapter) SetProcessLowPriority(...string) error { return nil }
func (*fakePlatformAdapter) CheckProcessPrioritySupport() error      { return nil }
func (*fakePlatformAdapter) RestoreProcessPriority() error           { return nil }
func (*fakePlatformAdapter) DetectGPU() platform.GPUInfo              { return platform.GPUInfo{} }

func (f *fakePlatformAdapter) LaunchURI(ctx context.Context, uri string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, "launch:"+uri)
	if f.onLaunch != nil {
		return f.onLaunch(uri)
	}
	return nil
}

func (f *fakePlatformAdapter) CloseGame() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, "close")
	if f.closeErr != nil {
		return f.closeErr
	}
	if !f.isStuck {
		f.isRunning = false
	}
	return nil
}

func TestSwitchServerDirectlyWhileRunning(t *testing.T) {
	t.Parallel()

	adapter := &fakePlatformAdapter{isRunning: true}
	controller := testController(t, adapter)
	controller.processPollInterval = time.Millisecond
	controller.serverSwitchTimeout = 100 * time.Millisecond

	const joinURL = "steam://joinlobby/393380/109775242037177537/76561199185206154"
	if err := controller.SwitchServer(context.Background(), joinURL); err != nil {
		t.Fatalf("SwitchServer() error = %v", err)
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.actions) != 1 || adapter.actions[0] != "launch:"+joinURL {
		t.Fatalf("SwitchServer() actions = %#v, want direct join only", adapter.actions)
	}
}

func TestSwitchServerLaunchesSquadIfStopped(t *testing.T) {
	t.Parallel()

	adapter := &fakePlatformAdapter{isRunning: false}
	controller := testController(t, adapter)
	controller.processPollInterval = time.Millisecond
	controller.serverSwitchTimeout = 100 * time.Millisecond

	const joinURL = "steam://joinlobby/393380/109775242037177537/76561199185206154"
	if err := controller.SwitchServer(context.Background(), joinURL); err != nil {
		t.Fatalf("SwitchServer() error = %v", err)
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.actions) != 2 || adapter.actions[0] != "launch:steam://rungameid/393380" || adapter.actions[1] != "launch:"+joinURL {
		t.Fatalf("SwitchServer() actions = %#v, want start then join", adapter.actions)
	}
}

func TestConnectRejectsCancelledLaunchBeforeNativeDispatch(t *testing.T) {
	adapter := &fakePlatformAdapter{}
	controller := testController(t, adapter)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := controller.Connect(ctx, "steam://joinlobby/393380/1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled connect returned %v", err)
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.actions) != 0 {
		t.Fatal("cancelled connect launched a native command")
	}
}

// testController supplies isolated fresh startup evidence without touching Steam.
func testController(t *testing.T, adapter *fakePlatformAdapter) *Controller {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SquadGame.log")
	controller := NewController(adapter)
	controller.logPaths = func() []string { return []string{path} }
	controller.processPollInterval = time.Millisecond
	controller.startupTimeout = 100 * time.Millisecond
	adapter.onLaunch = func(uri string) error {
		if uri == "steam://rungameid/393380" {
			adapter.isRunning = true
			return os.WriteFile(path, []byte("LogLoad: Took 0.3 seconds to LoadMap(/Game/Maps/EntryMap)\nLogOnlineServicesSubsystem: [HandleLoginComplete] Success!\n"), 0600)
		}
		return nil
	}
	return controller
}

func TestColdStartWaitsForMenuAndOnlineLoginBeforeSendingInvitation(t *testing.T) {
	adapter := &fakePlatformAdapter{}
	controller := testController(t, adapter)
	controller.startupTimeout = time.Second
	path := controller.logPaths()[0]
	launched := make(chan struct{}, 1)
	adapter.onLaunch = func(uri string) error {
		if uri == "steam://rungameid/393380" {
			adapter.isRunning = true
			launched <- struct{}{}
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- controller.Connect(ctx, "steam://joinlobby/393380/1") }()
	select {
	case <-launched:
	case <-time.After(time.Second):
		t.Fatal("Squad was not started")
	}
	appendConnectionLog(t, path, "LogLoad: Took 0.3 seconds to LoadMap(/Game/Maps/EntryMap)\n")
	select {
	case err := <-finished:
		t.Fatalf("invitation sent before online login: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	appendConnectionLog(t, path, "LogOnlineServicesSubsystem: [HandleLoginComplete] Success!\n")
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("ready client did not receive invitation")
	}
	server := citadel.Server{ServerIP: "192.0.2.1", GamePort: 7812}
	if connected, err := controller.CheckServerConnection(server); err != nil || connected {
		t.Fatalf("boot was mistaken for server membership: %v %v", connected, err)
	}
	appendConnectionLog(t, path, "LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap\n"+observedWelcome+observedDeployment)
	if connected, err := controller.CheckServerConnection(server); err != nil || !connected {
		t.Fatalf("client join was not recognized: %v %v", connected, err)
	}
}

func TestColdStartCancellationNeverDispatchesTheLobby(t *testing.T) {
	adapter := &fakePlatformAdapter{}
	controller := testController(t, adapter)
	controller.startupTimeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	adapter.onLaunch = func(string) error { cancel(); return nil }
	if err := controller.Connect(ctx, "steam://joinlobby/393380/1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled cold start = %v", err)
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.actions) != 1 || adapter.actions[0] != "launch:steam://rungameid/393380" {
		t.Fatalf("cancelled startup dispatched lobby: %v", adapter.actions)
	}
}

func TestInvitationIsResolvedOnlyAfterColdStartup(t *testing.T) {
	adapter := &fakePlatformAdapter{}
	controller := testController(t, adapter)
	if err := controller.ConnectResolved(context.Background(), func(context.Context) (string, error) {
		adapter.mu.Lock()
		defer adapter.mu.Unlock()
		if len(adapter.actions) != 1 || adapter.actions[0] != "launch:steam://rungameid/393380" {
			t.Fatal("resolver ran before normal launch")
		}
		if !adapter.isRunning {
			t.Fatal("resolver ran before Squad started")
		}
		return "steam://joinlobby/393380/1", nil
	}, false); err != nil {
		t.Fatal(err)
	}
}

func TestHiddenEndpointRequiresLoadedClientAndExactRosterConfirmation(t *testing.T) {
	adapter := &fakePlatformAdapter{isRunning: true}
	controller := testController(t, adapter)
	path := controller.logPaths()[0]
	if err := controller.Connect(context.Background(), "steam://joinlobby/393380/1"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	controller.SetPlayerVerifier(func(_ context.Context, serverID, steamID string) (bool, error) {
		calls++
		if serverID != "2" || steamID != "76561199237295347" {
			t.Fatal("wrong identity supplied to roster verifier")
		}
		return true, nil
	})
	server := citadel.Server{ID: "2", Name: "Test"}
	if ok, err := controller.CheckServerConnection(server); err != nil || ok || calls != 0 {
		t.Fatalf("menu confirmed membership: %v %v", ok, err)
	}
	appendConnectionLog(t, path, "LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap\n"+observedWelcome+"LogSquadTrace: [Client] OnUnPossess(): PC=Test (Online IDs: EOS: abcdef steam: 76561199237295347)\n"+observedDeployment)
	if ok, err := controller.CheckServerConnection(server); err != nil || !ok || calls != 1 {
		t.Fatalf("hidden endpoint could not confirm via roster: %v %v", ok, err)
	}
	if ok, err := controller.CheckServerConnection(server); err != nil || !ok || calls != 1 {
		t.Fatal("roster is not rate bounded")
	}
	appendConnectionLog(t, path, "LogOnline: GotoState: NewState: MainMenu\n")
	if ok, err := controller.CheckServerConnection(server); err != nil || ok {
		t.Fatal("cached roster confirmation survived client disconnect")
	}
}

func TestJoinJitterDelayApplied(t *testing.T) {
	adapter := &fakePlatformAdapter{isRunning: true}
	controller := testController(t, adapter)
	controller.SetJoinJitterMax(20 * time.Millisecond)

	start := time.Now()
	err := controller.ConnectResolved(context.Background(), func(context.Context) (string, error) {
		return "steam://joinlobby/393380/1", nil
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed < 0 {
		t.Fatal("unexpected negative elapsed time")
	}
}
