package citadel

import "fmt"

// CitadelHostIP is the authoritative public IP of the Citadel dedicated game server host.
const CitadelHostIP = "62.122.215.132"

// EndpointInfo describes network ports for a Citadel dedicated game server.
type EndpointInfo struct {
	ServerID  string
	GamePort  int
	QueryPort int
	BeaconUDP int
}

// KnownCitadelServers contains the authoritative endpoints for all production Citadel servers.
var KnownCitadelServers = map[string]EndpointInfo{
	"1": {ServerID: "1", GamePort: 7787, QueryPort: 21114, BeaconUDP: 15005}, // [RU] CITADEL [AAS/RAAS]
	"2": {ServerID: "2", GamePort: 7792, QueryPort: 21119, BeaconUDP: 15010}, // [RU] CITADEL [AAS+]
	"3": {ServerID: "3", GamePort: 7797, QueryPort: 21124, BeaconUDP: 15015}, // [RU] CITADEL [TRAINING]
	"4": {ServerID: "4", GamePort: 7802, QueryPort: 21129, BeaconUDP: 15020}, // [RU] CITADEL [EVENTS]
	"6": {ServerID: "6", GamePort: 7812, QueryPort: 21144, BeaconUDP: 15030}, // [RU] CITADEL [MIX]
}

// GetCitadelDirectConnectURL returns the authoritative steam://connect/<ip>:<game_port> URL.
// When dispatched via Steam, Unreal Engine 4 automatically connects the client directly,
// bypassing empty Steam lobby limitations when the server has 0 players.
func GetCitadelDirectConnectURL(serverID string) (string, bool) {
	if info, ok := KnownCitadelServers[serverID]; ok {
		return fmt.Sprintf("steam://connect/%s:%d", CitadelHostIP, info.GamePort), true
	}
	return "", false
}
