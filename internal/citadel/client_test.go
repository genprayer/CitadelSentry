package citadel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveJoinURLAcceptsThreePartLobbyWithPlayerSteamID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"ready",
			"method":"steam_joinlobby",
			"joinUrl":"steam://joinlobby/393380/109775244404557828/76561199624244404",
			"steamAppId":"393380"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	joinURL, err := client.ResolveJoinURL(context.Background(), "1")
	if err != nil {
		t.Fatalf("ResolveJoinURL() error = %v", err)
	}
	expected := "steam://joinlobby/393380/109775244404557828/76561199624244404"
	if joinURL != expected {
		t.Fatalf("ResolveJoinURL() = %q, want %q", joinURL, expected)
	}
}

func TestResolveJoinURLAcceptsSquadLobby(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"ready",
			"method":"steam_joinlobby",
			"joinUrl":"steam://joinlobby/393380/109775242037177537/76561199185206154",
			"steamAppId":"393380"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	joinURL, err := client.ResolveJoinURL(context.Background(), "1")
	if err != nil {
		t.Fatalf("ResolveJoinURL() error = %v", err)
	}
	if joinURL != "steam://joinlobby/393380/109775242037177537/76561199185206154" {
		t.Fatalf("ResolveJoinURL() = %q", joinURL)
	}
}

func TestResolveJoinURLAcceptsTwoPartLobby(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"ready",
			"method":"steam_joinlobby",
			"joinUrl":"steam://joinlobby/393380/109775242080666198",
			"steamAppId":"393380"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	joinURL, err := client.ResolveJoinURL(context.Background(), "1")
	if err != nil {
		t.Fatalf("ResolveJoinURL() error = %v", err)
	}
	if joinURL != "steam://joinlobby/393380/109775242080666198" {
		t.Fatalf("ResolveJoinURL() = %q", joinURL)
	}
}

func TestResolveJoinURLAcceptsDirectConnect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"ready",
			"method":"steam_connect",
			"joinUrl":"steam://connect/62.122.215.132:7787",
			"steamAppId":"393380"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	joinURL, err := client.ResolveJoinURL(context.Background(), "1")
	if err != nil {
		t.Fatalf("ResolveJoinURL() error = %v", err)
	}
	if joinURL != "steam://connect/62.122.215.132:7787" {
		t.Fatalf("ResolveJoinURL() = %q", joinURL)
	}
}

func TestResolveJoinURLRejectsUntrustedProtocol(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"ready",
			"method":"steam_joinlobby",
			"joinUrl":"https://example.com/payload",
			"steamAppId":"393380"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.ResolveJoinURL(context.Background(), "1"); err == nil {
		t.Fatal("ResolveJoinURL() error = nil, want validation error")
	}
}

func TestGetServersDoesNotFabricateOfflineFleet(t *testing.T) {
	t.Parallel()

	client := NewClient("http://127.0.0.1:1")
	client.httpClient = &http.Client{}
	if _, err := client.GetServers(context.Background()); err == nil {
		t.Fatal("GetServers() error = nil, want network error")
	}
}

func TestGetServersPreservesUnreadyServerStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"1","name":"One","status":"starting","players":100,"maxPlayers":100,"queue":0}]`))
	}))
	defer server.Close()
	fleet, err := NewClient(server.URL).GetServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(fleet) != 1 || fleet[0].Status != "starting" {
		t.Fatalf("unready server was reclassified as available: %v", fleet)
	}
}

func TestGetServersRejectsIncompleteOrAmbiguousFleet(t *testing.T) {
	valid := map[string]any{"id": "1", "name": "One", "status": "online", "players": 10, "maxPlayers": 100, "queue": 0}
	payloads := map[string]string{
		"null-fleet":    "null",
		"null-server":   "[null]",
		"duplicate-id":  `[{"id":"1","name":"One","status":"online","players":10,"maxPlayers":100,"queue":0},{"id":"1","name":"Two","status":"online","players":20,"maxPlayers":100,"queue":0}]`,
		"duplicate-key": `[{"id":"1","name":"One","status":"online","players":10,"players":20,"maxPlayers":100,"queue":0}]`,
	}
	for _, field := range []string{"id", "name", "status", "players", "maxPlayers", "queue"} {
		for _, isNull := range []bool{false, true} {
			row := make(map[string]any)
			for key, value := range valid {
				row[key] = value
			}
			delete(row, field)
			name := "missing-" + field
			if isNull {
				row[field] = nil
				name = "null-" + field
			}
			data, err := json.Marshal([]any{row})
			if err != nil {
				t.Fatal(err)
			}
			payloads[name] = string(data)
		}
	}
	for _, field := range []string{"players", "maxPlayers", "queue"} {
		row := make(map[string]any)
		for key, value := range valid {
			row[key] = value
		}
		row[field] = -1
		data, err := json.Marshal([]any{row})
		if err != nil {
			t.Fatal(err)
		}
		payloads["negative-"+field] = string(data)
	}
	for name, changes := range map[string]map[string]any{
		"numeric-id":           {"id": 1},
		"object-id":            {"id": map[string]any{"unexpected": "id"}},
		"empty-id":             {"id": ""},
		"padded-id":            {"id": " 1"},
		"unsafe-count":         {"players": int64(1 << 53)},
		"invalid-port":         {"port": 65536},
		"negative-port":        {"queryPort": -1},
		"case-mismatched-port": {"Port": 7787},
	} {
		row := make(map[string]any)
		for key, value := range valid {
			row[key] = value
		}
		for key, value := range changes {
			row[key] = value
		}
		data, err := json.Marshal([]any{row})
		if err != nil {
			t.Fatal(err)
		}
		payloads[name] = string(data)
	}
	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(payload)) }))
			defer server.Close()
			if _, err := NewClient(server.URL).GetServers(context.Background()); err == nil {
				t.Fatal("unverified fleet was accepted")
			}
		})
	}
}

func TestGetServersPreservesEmptyFleetAndNullableMetadata(t *testing.T) {
	for _, payload := range []string{"[]", `[{"id":"1","name":"","status":"unknown","players":0,"maxPlayers":0,"queue":0,"ip":null,"port":null,"queryPort":null,"mapName":null,"layer":null,"futureMetadata":{"value":true}}]`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(payload)) }))
		fleet, err := NewClient(server.URL).GetServers(context.Background())
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if payload == "[]" {
			if len(fleet) != 0 {
				t.Fatal("empty fleet was replaced")
			}
			continue
		}
		if len(fleet) != 1 || fleet[0].Name != "" || fleet[0].CurrentMap != "" || fleet[0].MaxPlayers != 0 || fleet[0].GamePort != 0 {
			t.Fatalf("unavailable metadata was replaced with invented values: %+v", fleet)
		}
	}
}

func TestClientDoesNotInventAnUnconfiguredEndpoint(t *testing.T) {
	client := NewClient("")
	if client.GetBaseURL() != "" {
		t.Fatal("unconfigured client substituted an unrelated API endpoint")
	}
}

func TestResolveJoinURLReportsWebsiteFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	if _, err := client.ResolveJoinURL(context.Background(), "1"); err == nil {
		t.Fatal("failed website lookup did not return an error")
	}
}

func TestResolveJoinURLPreservesUnavailableWebsiteResult(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/servers/2/connect" {
			t.Errorf("lookup used a different server path: %s", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"unavailable","method":"none","message":"Server connection is unavailable"}`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	joinURL, err := client.ResolveJoinURL(context.Background(), "2")
	if err == nil || joinURL != "" {
		t.Fatalf("unavailable target was replaced with a guessed server: %q, error %v", joinURL, err)
	}
}
