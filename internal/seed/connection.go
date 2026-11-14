package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"citadel-seeder/internal/citadel"
	"citadel-seeder/internal/game"
	"citadel-seeder/internal/logger"
)

const (
	maxSteamInviteRetries = 2
	steamInviteRetryDelay = 2 * time.Second
	maxCrashRetries       = 2
)

// monitorServer accounts only intervals for which the client remains connected
// to the selected server. Startup, menus, queues and travel do not consume time.
func (e *Engine) monitorServer(ctx context.Context, runID uint64, plan Plan, server citadel.Server) (serverOutcome, serverActivity, error) {
	var activity serverActivity
	var elapsed time.Duration
	var previousCheck time.Time
	wasConnected, hasObservedProcess := false, false
	connectionDeadline := time.Now().Add(e.connectionTimeout)
	progress := time.NewTicker(e.progressEvery)
	defer progress.Stop()
	status := time.NewTicker(e.statusEvery)
	defer status.Stop()
	isFirstCheck := true
	inviteRetries := 0
	crashRetries := 0
	for {
		shouldFetch := false
		if !isFirstCheck {
			select {
			case <-ctx.Done():
				return outcomeTimedOut, activity, ErrSeedStopped
			case <-e.skipServerCh:
				logger.Info("Skip signal received; advancing past server", "server", server.Name)
				return outcomeSkipped, activity, nil
			case <-progress.C:
			case <-status.C:
				shouldFetch = true
			}
		} else if e.skipServerCh != nil {
			select {
			case <-e.skipServerCh:
				logger.Info("Skip signal received on initial check; advancing past server", "server", server.Name)
				return outcomeSkipped, activity, nil
			default:
			}
		}
		isFirstCheck = false
		if ctx.Err() != nil {
			return outcomeTimedOut, activity, ErrSeedStopped
		}
		isRunning, err := e.game.CheckSquadRunning()
		if err != nil {
			return outcomeTimedOut, activity, fmt.Errorf("inspect Squad process: %w", err)
		}
		if !isRunning && hasObservedProcess {
			if plan.AutoRecoverCrash && crashRetries < maxCrashRetries && ctx.Err() == nil {
				crashRetries++
				logger.Warn("Squad process exited unexpectedly; attempting auto-recovery", "attempt", crashRetries, "max", maxCrashRetries, "server", server.Name)
				time.Sleep(2 * time.Second)
				resolve := func(ctx context.Context) (string, error) { return e.servers.ResolveJoinURL(ctx, server.ID) }
				if err := e.game.ConnectResolved(ctx, resolve, false); err != nil {
					return outcomeTimedOut, activity, fmt.Errorf("recover Squad process: %w", err)
				}
				hasObservedProcess = false
				connectionDeadline = time.Now().Add(e.connectionTimeout)
				continue
			}
			return outcomeTimedOut, activity, errors.New("squad process was closed")
		}
		hasObservedProcess = hasObservedProcess || isRunning
		isConnected := false
		if isRunning {
			isConnected, err = e.game.CheckServerConnection(server)
			if !isConnected {
				detectedID := e.game.GetObservedConnectedServerID()
				if detectedID != "" && detectedID != server.ID {
					logger.Info("Detected player connected to different Citadel server in-game; auto-switching active seeding target",
						"fromServer", server.Name, "toServerID", detectedID)
					newServer, fetchErr := e.fetchServer(ctx, detectedID)
					if fetchErr == nil {
						server = newServer
						e.updateServerTelemetry(runID, server)
						isConnected = true
					}
				}
			}
			if errors.Is(err, game.ErrSteamInviteBusy) && inviteRetries < maxSteamInviteRetries {
				inviteRetries++
				timer := time.NewTimer(steamInviteRetryDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return outcomeTimedOut, activity, ErrSeedStopped
				case <-e.skipServerCh:
					timer.Stop()
					return outcomeSkipped, activity, nil
				case <-timer.C:
				}
				resolve := func(ctx context.Context) (string, error) { return e.servers.ResolveJoinURL(ctx, server.ID) }
				if err := e.game.ConnectResolved(ctx, resolve, false); err != nil {
					return outcomeTimedOut, activity, err
				}
				previousCheck, wasConnected = time.Time{}, false
				continue
			}
			if err != nil {
				return outcomeTimedOut, activity, fmt.Errorf("confirm connection to %s: %w", server.Name, err)
			}
		}
		now := time.Now()
		if isConnected && !wasConnected {
			e.notifyConnectionState(true)
		}
		if !isConnected && wasConnected {
			e.notifyConnectionState(false)
			connectionDeadline = now.Add(e.connectionTimeout)
		}
		if isConnected && wasConnected {
			elapsed += now.Sub(previousCheck)
		}
		previousCheck, wasConnected = now, isConnected
		activity.hasObservedGame = activity.hasObservedGame || isConnected
		activity.duration = elapsed
		remaining := e.planDuration(plan) - elapsed
		if remaining < 0 {
			remaining = 0
		}
		remainingSeconds := int((remaining + time.Second - 1) / time.Second)
		if !e.update(runID, func(session *Session) {
			session.HasObservedGame = session.HasObservedGame || isRunning
			session.State = StateConnecting
			if isConnected {
				session.State = StateActive
				if session.StartedAt == nil {
					session.StartedAt = &now
				}
			}
			session.ElapsedSeconds = int(elapsed.Seconds())
			session.RemainingSeconds = &remainingSeconds
		}) {
			return outcomeTimedOut, activity, ErrSeedStopped
		}
		if !isConnected {
			if !now.Before(connectionDeadline) {
				return outcomeTimedOut, activity, fmt.Errorf("connection to %s was not confirmed before the connection deadline; seeding time was not counted", server.Name)
			}
			continue
		}
		if remaining == 0 {
			return outcomeTimedOut, activity, nil
		}
		if !shouldFetch {
			continue
		}
		current, err := e.fetchServer(ctx, server.ID)
		if err != nil {
			continue
		}
		server = current
		e.updateServerTelemetry(runID, server)
		if server.CurrentPlayers >= plan.TargetPlayerCount {
			return outcomeTargetReached, activity, nil
		}
	}
}
