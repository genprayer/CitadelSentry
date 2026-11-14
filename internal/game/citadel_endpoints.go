package game

import (
	"citadel-seeder/internal/citadel"
	"net"
	"strconv"
)

// CitadelHostIP is the authoritative IP address of the Citadel game server host.
const CitadelHostIP = citadel.CitadelHostIP

// CitadelEndpointInfo describes network ports for a Citadel dedicated server.
type CitadelEndpointInfo = citadel.EndpointInfo

// KnownCitadelServers contains the authoritative endpoints for all production Citadel servers.
var KnownCitadelServers = citadel.KnownCitadelServers

// ResolveCitadelServerByEndpoint maps an observed game endpoint ("62.122.215.132:7787")
// to the matching Citadel Server ID ("1"), or returns "" if unmapped.
func ResolveCitadelServerByEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ""
	}
	if host != CitadelHostIP {
		return ""
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ""
	}
	for id, info := range KnownCitadelServers {
		if info.GamePort == port {
			return id
		}
	}
	return ""
}

// GetCitadelServerEndpoint returns the host IP and game port for a known Citadel server ID.
func GetCitadelServerEndpoint(serverID string) (string, int, bool) {
	if info, ok := KnownCitadelServers[serverID]; ok {
		return CitadelHostIP, info.GamePort, true
	}
	return "", 0, false
}

// GetCitadelDirectConnectURL returns the authoritative steam://connect/<ip>:<game_port> URL.
func GetCitadelDirectConnectURL(serverID string) (string, bool) {
	return citadel.GetCitadelDirectConnectURL(serverID)
}
