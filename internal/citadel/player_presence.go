package citadel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"citadel-seeder/internal/fileutil"
)

var steamPlayerID = regexp.MustCompile(`^[0-9]{17}$`)

// HasServerPlayer checks the existing public squad roster by exact SteamID.
// A name match or total player count cannot establish a player's membership.
func (c *Client) HasServerPlayer(ctx context.Context, serverID, steamID string) (bool, error) {
	if serverID == "" || !steamPlayerID.MatchString(steamID) {
		return false, fmt.Errorf("server and local SteamID are required for connection confirmation")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.GetBaseURL()+"/api/servers/"+url.PathEscape(serverID)+"/squads", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Citadel-Sentry-Desktop")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("read server player roster: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("server player roster returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxServerResponseBytes+1))
	if err != nil {
		return false, err
	}
	if len(data) > maxServerResponseBytes {
		return false, fmt.Errorf("server player roster exceeded its size limit")
	}
	type player struct {
		SteamID string `json:"steamID"`
	}
	type squad struct {
		Players *[]player `json:"players"`
	}
	var roster struct {
		Team1 *[]squad `json:"team1"`
		Team2 *[]squad `json:"team2"`
	}
	var fields map[string]json.RawMessage
	if err := fileutil.DecodeJSONStrict(data, &fields); err != nil {
		return false, fmt.Errorf("decode server player roster: %w", err)
	}
	if err := json.Unmarshal(data, &roster); err != nil {
		return false, fmt.Errorf("decode server player roster fields: %w", err)
	}
	if roster.Team1 == nil || roster.Team2 == nil {
		return false, fmt.Errorf("server player roster is unavailable")
	}
	for _, team := range [][]squad{*roster.Team1, *roster.Team2} {
		for _, s := range team {
			if s.Players == nil {
				return false, fmt.Errorf("server squad players are unavailable")
			}
			for _, p := range *s.Players {
				if p.SteamID == steamID {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
