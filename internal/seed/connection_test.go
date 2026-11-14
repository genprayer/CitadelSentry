package seed

import (
	"context"
	"errors"
	"testing"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/game"
)

type confirmedConnectionGame struct {
	fakeGameController
	confirmedServer string
}

func (g *confirmedConnectionGame) CheckServerConnection(server citadel.Server) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.confirmedServer == server.ID, nil
}

func (g *confirmedConnectionGame) GetObservedConnectedServerID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.confirmedServer
}

func (g *confirmedConnectionGame) confirm(server string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.confirmedServer = server
}

func (g *confirmedConnectionGame) ConnectResolved(ctx context.Context, resolve func(context.Context) (string, error), shouldSwitch bool) error {
	if shouldSwitch {
		g.mu.Lock()
		g.confirmedServer = ""
		g.mu.Unlock()
	}
	return g.fakeGameController.ConnectResolved(ctx, resolve, shouldSwitch)
}

func startConnectionFixture(t *testing.T, game GameController, provider *fakeServerProvider) *Engine {
	t.Helper()
	engine := NewEngine(fakeSystemController{}, game, provider, fakeSettingsProvider{}, nil)
	engine.progressEvery = 2 * time.Millisecond
	engine.statusEvery = 5 * time.Millisecond
	engine.durationUnit = 80 * time.Millisecond
	engine.connectionTimeout = time.Second
	plan := Plan{ServerID: provider.servers[0].ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedDoNothing, TimeoutAction: TimeoutContinueNext}
	if err := engine.Start(context.Background(), plan, provider.servers); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Stop(); err != nil {
			t.Error(err)
		}
		waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	})
	return engine
}

func TestTimerAndQueueWaitForConfirmedPlayerConnectionOnEveryServer(t *testing.T) {
	servers := []citadel.Server{{ID: "one", Name: "One", Status: "online", MaxPlayers: 100}, {ID: "two", Name: "Two", Status: "online", MaxPlayers: 100}}
	game := &confirmedConnectionGame{}
	provider := &fakeServerProvider{servers: servers}
	engine := startConnectionFixture(t, game, provider)
	waitFor(t, time.Second, func() bool { return engine.GetSession().HasObservedGame })
	time.Sleep(120 * time.Millisecond) // Longer than this fixture's seed duration.
	session := engine.GetSession()
	if session.State != StateConnecting || session.StartedAt != nil || session.ElapsedSeconds != 0 || session.CurrentQueueIndex != 1 {
		t.Fatalf("unconfirmed launch consumed the mission: %+v", session)
	}
	confirmedAt := time.Now()
	game.confirm("one")
	waitFor(t, time.Second, func() bool { return engine.GetSession().State == StateActive })
	if start := engine.GetSession().StartedAt; start == nil || start.Before(confirmedAt) {
		t.Fatal("timer predates actual confirmation")
	}
	// Only confirmed time can trigger the timeout transition.
	waitFor(t, time.Second, func() bool { return engine.GetSession().CurrentQueueIndex == 2 && engine.GetSession().HasObservedGame })
	time.Sleep(120 * time.Millisecond)
	session = engine.GetSession()
	if session.State != StateConnecting || session.StartedAt != nil || session.ElapsedSeconds != 0 {
		t.Fatalf("first server membership authorized the second timer: %+v", session)
	}
	game.confirm("two")
	waitFor(t, time.Second, func() bool { return engine.GetSession().State == StateActive })
	provider.mu.Lock()
	provider.servers[1].CurrentPlayers = 50
	provider.mu.Unlock()
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	if engine.GetSession().State != StateTimedOut || engine.GetSession().SkippedServerCount != 1 {
		t.Fatal("mixed queue was not reported as timed out")
	}
	game.mu.Lock()
	defer game.mu.Unlock()
	if game.switchCount != 1 || game.closeCount != 1 {
		t.Fatal("transition did not disconnect the previous server once")
	}
}

func TestDisconnectionPausesAccountingAndCannotCompleteMission(t *testing.T) {
	server := citadel.Server{ID: "one", Name: "One", Status: "online", MaxPlayers: 100}
	game := &confirmedConnectionGame{confirmedServer: "one"}
	provider := &fakeServerProvider{servers: []citadel.Server{server}}
	engine := startConnectionFixture(t, game, provider)
	waitFor(t, time.Second, func() bool { return engine.GetSession().State == StateActive })
	game.confirm("")
	waitFor(t, time.Second, func() bool { return engine.GetSession().State == StateConnecting })
	before := engine.GetSession()
	provider.mu.Lock()
	provider.servers[0].CurrentPlayers = 50
	provider.mu.Unlock()
	time.Sleep(120 * time.Millisecond)
	after := engine.GetSession()
	if after.State != StateConnecting || after.ElapsedSeconds != before.ElapsedSeconds || *after.RemainingSeconds != *before.RemainingSeconds {
		t.Fatal("disconnected time or player target advanced the mission")
	}
	game.confirm("one")
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	if engine.GetSession().State != StateCompleted {
		t.Fatal("reconfirmed server did not complete")
	}
}

func TestRejectedInvitationFailsWithoutStartingOrAdvancingSeed(t *testing.T) {
	server := citadel.Server{ID: "one", Name: "One", Status: "online", MaxPlayers: 100}
	game := &fakeGameController{connectionError: errors.New("RedpointEOSRoomId rejected")}
	engine := startConnectionFixture(t, game, &fakeServerProvider{servers: []citadel.Server{server}})
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	session := engine.GetSession()
	if session.State != StateFailed || session.StartedAt != nil || session.ElapsedSeconds != 0 {
		t.Fatalf("rejected invite counted as seeding: %+v", session)
	}
	game.mu.Lock()
	defer game.mu.Unlock()
	if game.switchCount != 0 || game.closeCount != 0 {
		t.Fatal("connection failure executed a mission action")
	}
}

type busyInvitationGame struct {
	fakeGameController
	rejectedAttempts int
	hasRejected      chan struct{}
}

func (g *busyInvitationGame) CheckServerConnection(citadel.Server) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.connects) <= g.rejectedAttempts {
		select {
		case g.hasRejected <- struct{}{}:
		default:
		}
		return false, game.ErrSteamInviteBusy
	}
	return true, nil
}

func TestBusyInvitationsRetryWithinLimitWithoutCountingWaitingTime(t *testing.T) {
	for _, tc := range []struct {
		name             string
		rejectedAttempts int
		wantState        State
		wantAttempts     int
	}{
		{"recovers", 1, StateActive, 2},
		{"exhausted", 100, StateFailed, 1 + maxSteamInviteRetries},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := citadel.Server{ID: "one", Name: "One", Status: "online", MaxPlayers: 100}
			g := &busyInvitationGame{rejectedAttempts: tc.rejectedAttempts}
			engine := NewEngine(fakeSystemController{}, g, &fakeServerProvider{servers: []citadel.Server{server}}, fakeSettingsProvider{}, nil)
			engine.progressEvery = time.Millisecond
			plan := Plan{ServerID: server.ID, TargetPlayerCount: 50, MaxDurationMinutes: 1, PostSeedAction: PostSeedCloseSquad}
			if err := engine.Start(context.Background(), plan, []citadel.Server{server}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := engine.Stop(); err != nil {
					t.Error(err)
				}
				waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
			})
			waitFor(t, 6*time.Second, func() bool { return engine.GetSession().State == tc.wantState })
			if session := engine.GetSession(); session.ElapsedSeconds != 0 || (tc.wantState == StateFailed && session.StartedAt != nil) {
				t.Fatalf("ignored invitations counted as seeding: %+v", session)
			}
			g.mu.Lock()
			defer g.mu.Unlock()
			if len(g.connects) != tc.wantAttempts || g.closeCount != 0 || g.switchCount != 0 {
				t.Fatalf("unexpected attempts/actions: attempts=%d close=%d switch=%d", len(g.connects), g.closeCount, g.switchCount)
			}
		})
	}
}

func TestStopInterruptsBusyInvitationDelayWithoutRedispatch(t *testing.T) {
	server := citadel.Server{ID: "one", Name: "One", Status: "online", MaxPlayers: 100}
	g := &busyInvitationGame{rejectedAttempts: 100, hasRejected: make(chan struct{}, 1)}
	engine := startConnectionFixture(t, g, &fakeServerProvider{servers: []citadel.Server{server}})
	select {
	case <-g.hasRejected:
	case <-time.After(time.Second):
		t.Fatal("fixture did not reach the busy invitation delay")
	}
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return !engine.HasRunningWorker() })
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.connects) != 1 || g.closeCount != 0 {
		t.Fatal("stopped retry dispatched or executed a game action")
	}
}
