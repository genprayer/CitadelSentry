package game

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"citadel-seeder/internal/citadel"
)

func TestIncompatibleLobbyBuildRemainsTheCauseAfterSecondaryEOSFailure(t *testing.T) {
	var state connectionLogState
	state.consume("LogOnlineSession: Warning: STEAM: Removed incompatible build: ServerBuildUniqueId = 0x000a7f2c, GetBuildUniqueId() = 0x000a9aa0")
	if state.failure != nil {
		t.Fatal("an unrelated lobby search failure aborted the current connection")
	}
	state.consume("LogRedpointEOSPlatformIntegrationOSSv1: Error: Unable to find the 'RedpointEOSRoomId' attribute on the session invite")
	var buildError *SteamInviteBuildError
	if !errors.As(state.failure, &buildError) || buildError.Advertised != 687916 || buildError.Expected != 694944 {
		t.Fatalf("lost the observed incompatibility: %v", state.failure)
	}
	state.consume("LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap")
	if state.failure != nil {
		t.Fatal("a fresh connection retained the previous invitation rejection")
	}
}

func TestRejectedBuildFromEarlierSearchCannotExplainLaterInviteFailure(t *testing.T) {
	var state connectionLogState
	state.consume("LogOnlineSession: Warning: STEAM: Removed incompatible build: ServerBuildUniqueId = 0x000a7f2c, GetBuildUniqueId() = 0x000a9aa0")
	state.consume("LogOnlineSession: STEAM: Found 1 lobbies, finalizing the search")
	state.consume("LogRedpointEOSPlatformIntegrationOSSv1: Error: Unable to find the 'RedpointEOSRoomId' attribute on the session invite")
	var buildError *SteamInviteBuildError
	if state.failure == nil || errors.As(state.failure, &buildError) {
		t.Fatalf("earlier search contaminated the new invitation failure: %v", state.failure)
	}
}

func TestUnsuccessfulSteamInviteAcceptanceFailsWithoutCountingMembership(t *testing.T) {
	var state connectionLogState
	state.consume("[2026.10.23-23.08.12:432][848]LogRedpointEOSPlatformIntegrationOSSv1: Error: The runtime platform integration received an OnSessionUserInviteAccepted event from the underlying online subsystem, but bWasSuccessful was false so this event and invite will be ignored.")
	if state.failure == nil || state.isConnected {
		t.Fatal("the observed ignored invitation must fail without confirming membership")
	}
	state.consume("LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap")
	if state.failure != nil {
		t.Fatal("new travel retained a previous invitation failure")
	}
}

const observedWelcome = "LogNet: Welcomed by server (Level: /Game/Maps/Test, Game: Test)\r\n"
const observedDeployment = "LogSquad: Display: Loading requirement Deployment Menu finished\r\n"

func appendConnectionLog(t *testing.T, path, records string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(records)
	closeErr := f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}

func TestConnectionRequiresFreshWelcomeAndDeploymentForExactGameEndpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SquadGame.log")
	target := citadel.Server{ServerIP: "192.0.2.1", GamePort: 7812}
	joined := "LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap\r\n" + observedWelcome + observedDeployment
	appendConnectionLog(t, path, joined)
	log, err := newConnectionLog([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	check := func(want bool) {
		t.Helper()
		if err := log.poll(); err != nil {
			t.Fatal(err)
		}
		if got := log.isCaughtUp && log.state.matches(target); got != want {
			t.Fatalf("connected = %v, want %v; %+v", got, want, log.state)
		}
	}
	check(false) // A previous run is never confirmation of this dispatch.
	appendConnectionLog(t, path, "LogNet: Browse: 192.0.2.1:15030/Game/Maps/EntryMap\n"+observedWelcome+observedDeployment)
	check(false) // Beacon/query port is not the game connection.
	appendConnectionLog(t, path, "LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap\n")
	check(false)
	appendConnectionLog(t, path, observedWelcome)
	check(false) // Welcome precedes loading the playable client.
	appendConnectionLog(t, path, strings.TrimSuffix(observedDeployment, "\n"))
	check(false) // Wait for a complete record.
	appendConnectionLog(t, path, "\n")
	check(true)
	appendConnectionLog(t, path, "LogOnline: GotoState: NewState: MainMenu\n")
	check(false)
}

func TestConnectionLogRotationAndRegrowthCannotRetainPreviousMembership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SquadGame.log")
	log, err := newConnectionLog([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	appendConnectionLog(t, path, "Log file open, previous run\nLogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap\n"+observedWelcome+observedDeployment)
	if err := log.poll(); err != nil {
		t.Fatal(err)
	}
	if !log.state.isConnected {
		t.Fatal("fixture did not connect")
	}
	// Some games truncate in place and regrow past the cursor between polls.
	if err := os.WriteFile(path, []byte("Log file open, replacement run\n"+strings.Repeat("Unrelated new line\n", 100)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := log.poll(); err != nil {
		t.Fatal(err)
	}
	if log.state.isConnected {
		t.Fatal("truncation/regrowth retained old membership")
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	appendConnectionLog(t, path, "Log file open, third run\n"+observedDeployment)
	if err := log.poll(); err != nil {
		t.Fatal(err)
	}
	if log.state.isConnected {
		t.Fatal("replacement inherited an old welcome")
	}
}

func TestSteamInvitationRejectionIsAnErrorNotConnectionConfirmation(t *testing.T) {
	var state connectionLogState
	state.consume("LogRedpointEOSPlatformIntegrationOSSv1: Error: Unable to find the 'RedpointEOSRoomId' attribute on the session invite that was received via the runtime platform integration, or the attribute had a non-string value. This invite will be ignored.")
	if state.failure == nil || state.isConnected {
		t.Fatal("ignored invitation must be a visible connection failure")
	}
}

func TestChatMessagesCannotImpersonateConnectionRecords(t *testing.T) {
	var state connectionLogState
	for _, line := range []string{
		"LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap",
		"LogNet: Welcomed by server (Level: /Game/Maps/Test)",
		"LogSquad: Display: Loading requirement Deployment Menu finished",
	} {
		state.consume("[2026.10.23-20.07.17:192][733]LogSquad: Chat Msg Player[Test] Channel[4]: '" + line + "'")
	}
	if state.endpoint != "" || state.isConnected {
		t.Fatal("chat message forged a connection")
	}
}

func TestObservedClientIdentityAndBusySteamInvitation(t *testing.T) {
	var state connectionLogState
	state.consume("[2026.10.23-20.07.17:248][733]LogSquadTrace: [Client] OnUnPossess(): PC=genprayer (Online IDs: EOS: 0002447f5f7444ffb3e1d538adf673d6 steam: 76561199237295347)")
	if state.localSteamID != "76561199237295347" {
		t.Fatal("local client SteamID not captured")
	}
	state.consume("LogOnlineSession: Warning: STEAM: Invalid session or search already in progress when accepting invite.  Ignoring invite request.")
	if !state.wasSteamInviteBusy {
		t.Fatal("observed ignored invitation not recognized")
	}
	state.consume("LogNet: Browse: 192.0.2.1:7812/Game/Maps/EntryMap")
	if state.wasSteamInviteBusy {
		t.Fatal("fresh travel inherited previous invitation rejection")
	}
}
