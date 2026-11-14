package game

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"citadel-seeder/internal/citadel"
)

var squadLogRecord = regexp.MustCompile(`^(\[[0-9]{4}\.[0-9]{2}\.[0-9]{2}-[0-9]{2}\.[0-9]{2}\.[0-9]{2}:[0-9]{3}\]\[ *[0-9]+\])?(Log[A-Za-z0-9_]+:.*)$`)
var localPlayerRecord = regexp.MustCompile(`^LogSquadTrace: \[(Client|Standalone)\] .*\(Online IDs: EOS: [a-fA-F0-9]+ steam: ([0-9]{17})\)`)
var incompatibleLobbyBuild = regexp.MustCompile(`^LogOnlineSession: Warning: STEAM: Removed incompatible build: ServerBuildUniqueId = (0x[0-9a-fA-F]+), GetBuildUniqueId\(\) = (0x[0-9a-fA-F]+)`)

var ErrSteamInviteBusy = errors.New("Squad ignored the Steam invitation because a session search is still in progress")

// SteamInviteBuildError identifies a lobby filtered before EOS routing is read.
type SteamInviteBuildError struct {
	Advertised uint32
	Expected   uint32
}

// Error describes the observed invitation/client compatibility mismatch.
func (err *SteamInviteBuildError) Error() string {
	return fmt.Sprintf("Squad rejected the Steam lobby build: invitation advertises %d but the client requires %d", err.Advertised, err.Expected)
}

const (
	connectionLogReadLimit  = 1 << 20
	connectionLogLineLimit  = 64 << 10
	connectionLogHeaderSize = 128
)

// connectionLog reads only new records from the current client log. A beacon,
// invitation, process or map-loading request alone never confirms membership.
type connectionLog struct {
	paths      []string
	cursors    map[string]*connectionLogCursor
	current    string
	state      connectionLogState
	isCaughtUp bool
}

type connectionLogCursor struct {
	info    os.FileInfo
	offset  int64
	header  []byte
	pending []byte
}

type connectionLogState struct {
	endpoint           string
	isWelcomed         bool
	isConnected        bool
	isMenuLoaded       bool
	isAuthenticated    bool
	failure            error
	localSteamID       string
	wasSteamInviteBusy bool
	rejectedLobbyBuild *SteamInviteBuildError
}

func squadLogPaths() []string {
	paths := []string{}
	seen := map[string]bool{}
	for _, configPath := range getSquadConfigCandidates() {
		base := filepath.Dir(filepath.Dir(filepath.Dir(configPath)))
		appendSteamPath(&paths, seen, filepath.Join(base, "Logs", "SquadGame.log"))
	}
	return paths
}

func newConnectionLog(paths []string) (*connectionLog, error) {
	log := &connectionLog{paths: paths, cursors: map[string]*connectionLogCursor{}}
	for _, path := range paths {
		file, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("open Squad connection log: %w", err)
		}
		info, err := file.Stat()
		header := make([]byte, connectionLogHeaderSize)
		n, readErr := file.ReadAt(header, 0)
		closeErr := file.Close()
		if err != nil {
			return nil, err
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("Squad connection log is not a regular file")
		}
		log.cursors[path] = &connectionLogCursor{info: info, offset: info.Size(), header: header[:n]}
	}
	return log, nil
}

func (log *connectionLog) poll() error {
	var selected string
	var newest os.FileInfo
	for _, path := range log.paths {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect Squad connection log: %w", err)
		}
		if !info.Mode().IsRegular() {
			return errors.New("Squad connection log is not a regular file")
		}
		if newest == nil || info.ModTime().After(newest.ModTime()) {
			selected, newest = path, info
		}
	}
	if newest == nil {
		log.state = connectionLogState{}
		return nil
	}
	if selected != log.current {
		log.current = selected
		log.state = connectionLogState{}
	}
	file, err := os.Open(selected)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header := make([]byte, connectionLogHeaderSize)
	n, err := file.ReadAt(header, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	header = header[:n]
	cursor := log.cursors[selected]
	if cursor == nil || !os.SameFile(cursor.info, info) || info.Size() < cursor.offset ||
		!bytes.HasPrefix(header, cursor.header) {
		cursor = &connectionLogCursor{info: info, header: header}
		log.cursors[selected] = cursor
		log.state = connectionLogState{}
	}
	chunk := make([]byte, connectionLogReadLimit)
	n, err = file.ReadAt(chunk, cursor.offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	cursor.offset += int64(n)
	data := append(cursor.pending, chunk[:n]...)
	for {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			break
		}
		if end > connectionLogLineLimit {
			return errors.New("Squad connection log record exceeds its size limit")
		}
		log.state.consume(strings.TrimSuffix(string(data[:end]), "\r"))
		data = data[end+1:]
	}
	if len(data) > connectionLogLineLimit {
		return errors.New("Squad connection log record exceeds its size limit")
	}
	cursor.pending = append(cursor.pending[:0], data...)
	// Catch up before trusting the state: a later record may already disconnect it.
	log.isCaughtUp = cursor.offset >= info.Size()
	cursor.header = header
	return nil
}

func (state *connectionLogState) consume(line string) {
	parts := squadLogRecord.FindStringSubmatch(line)
	if parts == nil {
		return
	}
	line = parts[2]
	if identity := localPlayerRecord.FindStringSubmatch(line); identity != nil {
		state.localSteamID = identity[2]
	}
	if strings.HasPrefix(line, "LogNet: Browse: ") {
		value := strings.TrimSpace(strings.TrimPrefix(line, "LogNet: Browse: "))
		parsed, err := url.Parse("unreal://" + value)
		state.endpoint, state.isConnected, state.isWelcomed = "", false, false
		state.failure = nil
		state.rejectedLobbyBuild = nil
		state.wasSteamInviteBusy = false
		if err == nil && parsed.Port() != "" && net.ParseIP(parsed.Hostname()) != nil {
			state.endpoint = parsed.Host
		}
	}
	if strings.HasPrefix(line, "LogNet: Welcomed by server (") && state.endpoint != "" {
		state.isWelcomed = true
	}
	if strings.HasPrefix(line, "LogSquad: Display: Loading requirement Deployment Menu finished") && state.isWelcomed {
		state.isConnected = true
	}
	if strings.HasPrefix(line, "LogLoad: Took ") && strings.Contains(line, "LoadMap(/Game/Maps/EntryMap)") {
		state.isMenuLoaded = true
	}
	if strings.HasPrefix(line, "LogOnlineServicesSubsystem: [HandleLoginComplete] Success!") {
		state.isAuthenticated = true
	}
	if strings.HasPrefix(line, "LogOnlineSession: Warning: STEAM: Invalid session or search already in progress when accepting invite.") {
		state.wasSteamInviteBusy = true
	}
	if strings.HasPrefix(line, "LogOnlineSession: STEAM: Found ") && strings.HasSuffix(line, " lobbies, finalizing the search") {
		state.rejectedLobbyBuild = nil
	}
	if builds := incompatibleLobbyBuild.FindStringSubmatch(line); builds != nil {
		advertised, advertisedErr := strconv.ParseUint(builds[1][2:], 16, 32)
		expected, expectedErr := strconv.ParseUint(builds[2][2:], 16, 32)
		if advertisedErr == nil && expectedErr == nil {
			state.rejectedLobbyBuild = &SteamInviteBuildError{Advertised: uint32(advertised), Expected: uint32(expected)}
		}
	}
	if strings.HasPrefix(line, "LogOnline: GotoState: NewState: MainMenu") ||
		(strings.HasPrefix(line, "LogNet: World NetDriver shutdown") && strings.Contains(line, "[GameNetDriver]")) ||
		(strings.HasPrefix(line, "LogNet: UNetConnection::Close:") && strings.Contains(line, "Name:GameNetDriver")) {
		state.endpoint, state.isConnected, state.isWelcomed = "", false, false
	}
	if state.failure == nil && strings.HasPrefix(line, "LogRedpointEOSPlatformIntegrationOSSv1: Error: Unable to find the 'RedpointEOSRoomId'") {
		state.failure = errors.New("Squad rejected the Steam invitation: RedpointEOSRoomId is absent or is not a string")
		if state.rejectedLobbyBuild != nil {
			state.failure = state.rejectedLobbyBuild
		}
	}
	if state.failure == nil && strings.HasPrefix(line, "LogRedpointEOSPlatformIntegrationOSSv1: Error: The runtime platform integration received an OnSessionUserInviteAccepted event from the underlying online subsystem, but bWasSuccessful was false") {
		state.failure = errors.New("Squad could not resolve the Steam lobby invitation and ignored it; inspect SquadGame.log")
	}
	if strings.HasPrefix(line, "LogNet: NetworkFailure:") || strings.HasPrefix(line, "LogNet: TravelFailure:") {
		state.isConnected = false
		state.failure = errors.New("Squad reported a network or travel failure; inspect SquadGame.log")
	}
}

func (state connectionLogState) matches(server citadel.Server) bool {
	host, port, err := net.SplitHostPort(state.endpoint)
	if err != nil || !state.isConnected {
		return false
	}
	actualIP, targetIP := net.ParseIP(host), net.ParseIP(server.ServerIP)
	return actualIP != nil && targetIP != nil && actualIP.Equal(targetIP) && port == strconv.Itoa(server.GamePort)
}
