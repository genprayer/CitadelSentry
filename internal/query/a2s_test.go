package query

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestQueryA2SRejectsUnknownEndpointAndTimeout(t *testing.T) {
	for _, test := range []struct {
		name    string
		host    string
		timeout time.Duration
		message string
	}{
		{name: "missing-host", timeout: time.Millisecond, message: "host"},
		{name: "unknown-timeout", host: "127.0.0.1", message: "timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := QueryA2S(test.host, 1, test.timeout); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("invalid input was replaced with an assumed value: %v", err)
			}
		})
	}
}

func TestQueryA2SRejectsTruncatedChallengeBeforeResending(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	result := make(chan error, 1)
	go func() {
		buffer := make([]byte, 256)
		if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			result <- err
			return
		}
		_, peer, err := server.ReadFromUDP(buffer)
		if err != nil {
			result <- err
			return
		}
		_, err = server.WriteToUDP([]byte{0xff, 0xff, 0xff, 0xff, 0x41, 0x01}, peer)
		result <- err
	}()
	port := server.LocalAddr().(*net.UDPAddr).Port
	_, queryErr := QueryA2S("127.0.0.1", port, 50*time.Millisecond)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if queryErr == nil || !strings.HasPrefix(queryErr.Error(), "failed to read challenge:") {
		t.Fatalf("partial challenge was padded and retried instead of rejected: %v", queryErr)
	}
}
