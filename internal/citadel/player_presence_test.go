package citadel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicRosterRequiresExactSteamIdentityOnSelectedServer(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		want       bool
		wantError  bool
	}{
		{"present", `{"team1":[{"id":"unassigned","players":[{"name":"Test","steamID":"76561199237295347"}]}],"team2":[]}`, true, false},
		{"same name different ID", `{"team1":[{"players":[{"name":"Test","steamID":"76561199237295348"}]}],"team2":[]}`, false, false},
		{"absent", `{"team1":[],"team2":[]}`, false, false},
		{"unavailable", `{"players":0}`, false, true},
		{"null team", `{"team1":null,"team2":[]}`, false, true},
		{"missing squad players", `{"team1":[{}],"team2":[]}`, false, true},
		{"duplicate field", `{"team1":[],"team1":[],"team2":[]}`, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/servers/2/squads" {
					t.Errorf("wrong server path: %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			got, err := NewClient(server.URL).HasServerPlayer(context.Background(), "2", "76561199237295347")
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("presence=%v error=%v", got, err)
			}
		})
	}
}
