package citadel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"citadel-seeder/internal/logger"
)

const (
	requestTimeout         = 8 * time.Second
	maxServerResponseBytes = 4 << 20
	squadSteamAppID        = "393380"
	steamJoinLobbyMethod   = "steam_joinlobby"
	steamConnectMethod     = "steam_connect"
)

// Server represents a Citadel Squad server.
type Server struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	ServerIP        string   `json:"serverIp"`
	GamePort        int      `json:"gamePort"`
	QueryPort       int      `json:"queryPort"`
	CurrentPlayers  int      `json:"currentPlayers"`
	MaxPlayers      int      `json:"maxPlayers"`
	PublicQueue     int      `json:"publicQueue"`
	CurrentMap      string   `json:"currentMap"`
	CurrentLayer    string   `json:"currentLayer"`
	GameMode        string   `json:"gameMode,omitempty"`
	NextLayer       string   `json:"nextLayer,omitempty"`
	TeamOne         string   `json:"teamOne,omitempty"`
	TeamTwo         string   `json:"teamTwo,omitempty"`
	Status          string   `json:"status"` // "online", "seeding", "offline", "full"
	IsSeedingActive bool     `json:"isSeedingActive"`
	TargetSeedCount int      `json:"targetSeedCount"`
	PingMs          int      `json:"pingMs,omitempty"`
	Region          string   `json:"region,omitempty"`
	Rules           []string `json:"rules,omitempty"`
}

// UserProfile represents an authenticated Citadel user.
type UserProfile struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	SteamID64      string `json:"steam_id_64,omitempty"`
	DiscordID      string `json:"discord_id,omitempty"`
	IsSeedVerified bool   `json:"is_seed_verified"`
}

type cachedConnect struct {
	conn      *ServerConnect
	expiresAt time.Time
}

// Client interacts with the Citadel WebProject API.
type Client struct {
	mu             sync.RWMutex
	baseURL        string
	httpClient     *http.Client
	connectCacheMu sync.RWMutex
	connectCache   map[string]cachedConnect
}

// NewClient creates a new Citadel Web API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		connectCache: make(map[string]cachedConnect),
	}
}

// SetBaseURL updates the target base URL.
func (c *Client) SetBaseURL(url string) {
	if url == "" {
		return
	}

	c.mu.Lock()
	c.baseURL = strings.TrimRight(url, "/")
	c.mu.Unlock()
}

// GetBaseURL returns the configured base URL.
func (c *Client) GetBaseURL() string {
	c.mu.RLock()
	baseURL := c.baseURL
	c.mu.RUnlock()

	return baseURL
}

// ServerConnect represents the connection metadata for a server.
type ServerConnect struct {
	Status          string `json:"status"`
	Method          string `json:"method"`
	Message         string `json:"message,omitempty"`
	JoinURL         string `json:"joinUrl,omitempty"`
	SteamAppID      string `json:"steamAppId,omitempty"`
	SteamLobbyID    string `json:"steamLobbyId,omitempty"`
	FallbackAddress string `json:"fallbackAddress,omitempty"`
	Source          string `json:"source,omitempty"`
	Confidence      string `json:"confidence,omitempty"`
	ExpiresAt       string `json:"expiresAt,omitempty"`
}

type rawServerResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IP            *string `json:"ip"`
	Port          *int    `json:"port"`
	QueryPort     *int    `json:"queryPort"`
	MapName       *string `json:"mapName"`
	LocalMapImage *string `json:"localMapImage"`
	Players       int     `json:"players"`
	MaxPlayers    int     `json:"maxPlayers"`
	Status        string  `json:"status"`
	GameMode      *string `json:"gameMode"`
	Layer         *string `json:"layer"`
	Queue         int     `json:"queue"`
	Team1         *string `json:"team1"`
	Team2         *string `json:"team2"`
}

// GetServers retrieves the authoritative public fleet from the Citadel Web API.
func (c *Client) GetServers(ctx context.Context) ([]Server, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.GetBaseURL()+"/api/servers/sentry", nil)
	if err != nil {
		return nil, fmt.Errorf("create servers request: %w", err)
	}
	req.Header.Set("User-Agent", "Citadel-Sentry-Desktop")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Citadel servers: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request Citadel servers: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxServerResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Citadel servers response: %w", err)
	}
	if len(body) > maxServerResponseBytes {
		return nil, fmt.Errorf("Citadel servers response exceeded %d bytes", maxServerResponseBytes)
	}

	rawList, err := decodeServerFleet(body)
	if err != nil {
		return nil, fmt.Errorf("decode Citadel servers response: %w", err)
	}

	servers := make([]Server, 0, len(rawList))
	for _, r := range rawList {
		idStr := r.ID
		ip := ""
		if r.IP != nil && *r.IP != "" {
			ip = *r.IP
		}
		port := 0
		if r.Port != nil && *r.Port > 0 {
			port = *r.Port
		}
		qPort := 0
		if r.QueryPort != nil {
			qPort = *r.QueryPort
		}
		mapName := ""
		if r.MapName != nil {
			mapName = *r.MapName
		}
		layer := ""
		if r.Layer != nil {
			layer = *r.Layer
		}
		mode := ""
		if r.GameMode != nil {
			mode = *r.GameMode
		}
		t1 := ""
		if r.Team1 != nil {
			t1 = *r.Team1
		}
		t2 := ""
		if r.Team2 != nil {
			t2 = *r.Team2
		}

		maxPlayers := r.MaxPlayers

		status := strings.ToLower(r.Status)
		if status == "" {
			status = "unknown"
		}

		isSeeding := status == "seeding"
		if (status == "online" || status == "seeding") && r.Players >= maxPlayers && maxPlayers > 0 {
			status = "full"
			isSeeding = false
		}

		servers = append(servers, Server{
			ID:              idStr,
			Name:            r.Name,
			ServerIP:        ip,
			GamePort:        port,
			QueryPort:       qPort,
			CurrentPlayers:  r.Players,
			MaxPlayers:      maxPlayers,
			PublicQueue:     r.Queue,
			CurrentMap:      mapName,
			CurrentLayer:    layer,
			GameMode:        mode,
			TeamOne:         t1,
			TeamTwo:         t2,
			Status:          status,
			IsSeedingActive: isSeeding,
		})
	}

	return servers, nil
}

// GetServerConnect queries the SquadBrowser dynamic join endpoint for a server.
func (c *Client) GetServerConnect(ctx context.Context, serverID string) (*ServerConnect, error) {
	c.connectCacheMu.RLock()
	cached, found := c.connectCache[serverID]
	c.connectCacheMu.RUnlock()
	if found && time.Now().Before(cached.expiresAt) {
		logger.Debug("Using cached server connect URL", "serverID", serverID)
		return cached.conn, nil
	}

	endpoint := fmt.Sprintf("%s/api/servers/%s/connect", c.GetBaseURL(), url.PathEscape(serverID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create server connect request: %w", err)
	}
	req.Header.Set("User-Agent", "Citadel-Sentry-Desktop")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query server connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit reached (HTTP 429). SquadBrowser limits /join-link to 50 requests/hour; please wait before retrying")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query server connect: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxServerResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read server connect response: %w", err)
	}
	if len(body) > maxServerResponseBytes {
		return nil, fmt.Errorf("server connect response exceeded %d bytes", maxServerResponseBytes)
	}
	var conn ServerConnect
	if err := json.Unmarshal(body, &conn); err != nil {
		return nil, fmt.Errorf("decode server connect response: %w", err)
	}

	c.connectCacheMu.Lock()
	if c.connectCache == nil {
		c.connectCache = make(map[string]cachedConnect)
	}
	c.connectCache[serverID] = cachedConnect{
		conn:      &conn,
		expiresAt: time.Now().Add(85 * time.Second),
	}
	c.connectCacheMu.Unlock()

	return &conn, nil
}

// ResolveJoinURL uses the website's authoritative server connection metadata.
// It retrieves the validated Steam joinlobby URI provided by SquadBrowser / Citadel API,
// ensuring compatibility with OWI's lobby matchmaking system.
func (c *Client) ResolveJoinURL(ctx context.Context, serverID string) (string, error) {
	logger.Info("Resolving join URL via website API", "serverID", serverID)
	conn, err := c.GetServerConnect(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("resolve join URL for server %s: %w", serverID, err)
	}
	if conn.Status != "ready" {
		message := strings.TrimSpace(conn.Message)
		if message == "" {
			message = "automatic connection is unavailable"
		}
		return "", fmt.Errorf("resolve join URL for server %s: %s", serverID, message)
	}
	if conn.SteamAppID != "" && conn.SteamAppID != squadSteamAppID {
		return "", fmt.Errorf("resolve join URL for server %s: unexpected Steam app %s", serverID, conn.SteamAppID)
	}
	if conn.JoinURL == "" {
		return "", fmt.Errorf("resolve join URL for server %s: missing join URL", serverID)
	}
	if err := validateSquadJoinURL(conn.JoinURL); err != nil {
		return "", fmt.Errorf("resolve join URL for server %s: %w", serverID, err)
	}
	logger.Info("Successfully resolved join URL via website API", "serverID", serverID, "joinURL", conn.JoinURL)
	return conn.JoinURL, nil
}

func validateSquadJoinURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse join URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "steam") {
		return fmt.Errorf("unsupported join URL protocol")
	}

	if strings.EqualFold(parsed.Host, "connect") {
		endpoint := strings.Trim(parsed.Path, "/")
		host, portStr, err := net.SplitHostPort(endpoint)
		if err != nil {
			return fmt.Errorf("invalid connect endpoint: %w", err)
		}
		if net.ParseIP(host) == nil {
			return fmt.Errorf("invalid host IP in connect endpoint")
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port in connect endpoint")
		}
		return nil
	}

	if strings.EqualFold(parsed.Host, "joinlobby") {
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("join URL contains unsupported components")
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) < 2 || len(parts) > 3 || parts[0] != squadSteamAppID {
			return fmt.Errorf("join URL does not target Squad")
		}
		for _, value := range parts[1:] {
			if _, err := strconv.ParseUint(value, 10, 64); err != nil {
				return fmt.Errorf("join URL contains invalid lobby identifier")
			}
		}
		return nil
	}

	return fmt.Errorf("unsupported join URL target: %s", parsed.Host)
}
