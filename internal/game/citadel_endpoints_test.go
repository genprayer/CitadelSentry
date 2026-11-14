package game

import (
	"testing"
)

func TestResolveCitadelServerByEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		wantID   string
	}{
		{"62.122.215.132:7787", "1"},
		{"62.122.215.132:7792", "2"},
		{"62.122.215.132:7797", "3"},
		{"62.122.215.132:7802", "4"},
		{"62.122.215.132:7812", "6"},
		{"62.122.215.132:9999", ""},
		{"127.0.0.1:7787", ""},
		{"invalid", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := ResolveCitadelServerByEndpoint(tt.endpoint)
		if got != tt.wantID {
			t.Errorf("ResolveCitadelServerByEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.wantID)
		}
	}
}

func TestGetCitadelServerEndpoint(t *testing.T) {
	ip, port, ok := GetCitadelServerEndpoint("1")
	if !ok || ip != CitadelHostIP || port != 7787 {
		t.Errorf("GetCitadelServerEndpoint(1) = (%q, %d, %v), want (%q, 7787, true)", ip, port, ok, CitadelHostIP)
	}

	ip6, port6, ok6 := GetCitadelServerEndpoint("6")
	if !ok6 || ip6 != CitadelHostIP || port6 != 7812 {
		t.Errorf("GetCitadelServerEndpoint(6) = (%q, %d, %v), want (%q, 7812, true)", ip6, port6, ok6, CitadelHostIP)
	}

	_, _, okUnknown := GetCitadelServerEndpoint("99")
	if okUnknown {
		t.Errorf("GetCitadelServerEndpoint(99) returned ok=true")
	}
}
