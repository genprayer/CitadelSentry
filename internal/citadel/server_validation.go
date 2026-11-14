package citadel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"citadel-seeder/internal/fileutil"
)

// decodeServerFleet verifies producer fields before any zero values become telemetry.
func decodeServerFleet(data []byte) ([]rawServerResponse, error) {
	var fields []map[string]json.RawMessage
	if err := fileutil.DecodeJSONStrict(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("server fleet must be a JSON array")
	}
	for index, row := range fields {
		if row == nil {
			return nil, fmt.Errorf("server %d must be an object", index)
		}
		for key := range row {
			for _, canonical := range []string{"id", "name", "status", "players", "maxPlayers", "queue", "ip", "port", "queryPort", "mapName", "localMapImage", "gameMode", "layer", "team1", "team2"} {
				if key != canonical && strings.EqualFold(key, canonical) {
					return nil, fmt.Errorf("server %d has noncanonical field %s", index, key)
				}
			}
		}
		for _, key := range []string{"id", "name", "status", "players", "maxPlayers", "queue"} {
			value, exists := row[key]
			if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("server %d has no explicit %s", index, key)
			}
		}
	}
	var servers []rawServerResponse
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, err
	}
	seenIDs := make(map[string]bool, len(servers))
	for index, server := range servers {
		if server.ID == "" || server.ID != strings.TrimSpace(server.ID) || seenIDs[server.ID] {
			return nil, fmt.Errorf("server %d has an empty, ambiguous or repeated id", index)
		}
		seenIDs[server.ID] = true
		for key, count := range map[string]int{"players": server.Players, "maxPlayers": server.MaxPlayers, "queue": server.Queue} {
			if count < 0 || uint64(count) > (1<<53-1) {
				return nil, fmt.Errorf("server %s has invalid %s", server.ID, key)
			}
		}
		for key, port := range map[string]*int{"port": server.Port, "queryPort": server.QueryPort} {
			if port != nil && (*port < 0 || *port > 65535) {
				return nil, fmt.Errorf("server %s has invalid %s", server.ID, key)
			}
		}
	}
	return servers, nil
}
