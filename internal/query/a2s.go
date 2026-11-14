package query

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// ServerInfo holds real-time telemetry retrieved via Steam A2S Query.
type ServerInfo struct {
	Name       string        `json:"name"`
	Map        string        `json:"map"`
	Players    int           `json:"players"`
	MaxPlayers int           `json:"maxPlayers"`
	Ping       time.Duration `json:"ping"`
}

// QueryA2S performs a lightweight, direct UDP A2S_INFO query to the game server.
func QueryA2S(ip string, queryPort int, timeout time.Duration) (*ServerInfo, error) {
	if strings.TrimSpace(ip) == "" {
		return nil, fmt.Errorf("query host is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("query timeout must be greater than zero")
	}
	if queryPort <= 0 {
		return nil, fmt.Errorf("invalid query port: %d", queryPort)
	}

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, strconv.Itoa(queryPort)))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address %s:%d: %w", ip, queryPort, err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP %s: %w", addr.String(), err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set query deadline: %w", err)
	}

	start := time.Now()

	// Initial A2S_INFO request payload
	queryPacket := []byte("\xFF\xFF\xFF\xFFTSource Engine Query\x00")
	if _, err := conn.Write(queryPacket); err != nil {
		return nil, fmt.Errorf("failed to write A2S_INFO packet: %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read A2S response: %w", err)
	}

	r := bytes.NewReader(buf[:n])

	var prefix int32
	if err := binary.Read(r, binary.LittleEndian, &prefix); err != nil || prefix != -1 {
		return nil, fmt.Errorf("invalid A2S response header")
	}

	headerByte, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read header byte: %w", err)
	}

	// Handle S2C_CHALLENGE response (0x41 = 'A')
	if headerByte == 0x41 {
		challenge := make([]byte, 4)
		if _, err := io.ReadFull(r, challenge); err != nil {
			return nil, fmt.Errorf("failed to read challenge: %w", err)
		}

		// Resend A2S_INFO with challenge appended
		resend := append([]byte("\xFF\xFF\xFF\xFFTSource Engine Query\x00"), challenge...)
		if _, err := conn.Write(resend); err != nil {
			return nil, fmt.Errorf("failed to write challenged A2S packet: %w", err)
		}

		n, err = conn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read challenged A2S response: %w", err)
		}

		r = bytes.NewReader(buf[:n])
		if err := binary.Read(r, binary.LittleEndian, &prefix); err != nil || prefix != -1 {
			return nil, fmt.Errorf("invalid challenged A2S response header")
		}

		headerByte, err = r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read challenged header byte: %w", err)
		}
	}

	// Header byte for A2S_INFO reply must be 0x49 ('I')
	if headerByte != 0x49 {
		return nil, fmt.Errorf("unexpected A2S response type: 0x%02x (expected 0x49)", headerByte)
	}

	ping := time.Since(start)

	// Read protocol
	if _, err := r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read A2S protocol: %w", err)
	}

	// Read Server Name (null-terminated string)
	name, err := readNullTerminatedString(r)
	if err != nil {
		return nil, err
	}

	// Read Map (null-terminated string)
	mapName, err := readNullTerminatedString(r)
	if err != nil {
		return nil, err
	}

	// Read Folder (null-terminated string, e.g. "Squad")
	_, err = readNullTerminatedString(r)
	if err != nil {
		return nil, err
	}

	// Read Game (null-terminated string, e.g. "Squad")
	_, err = readNullTerminatedString(r)
	if err != nil {
		return nil, err
	}

	// Read Steam App ID (uint16)
	var appID uint16
	if err := binary.Read(r, binary.LittleEndian, &appID); err != nil {
		return nil, fmt.Errorf("read A2S application identifier: %w", err)
	}

	// Read Players count (uint8)
	playersByte, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read players count: %w", err)
	}

	// Read Max Players count (uint8)
	maxPlayersByte, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read max players count: %w", err)
	}

	return &ServerInfo{
		Name:       strings.TrimSpace(name),
		Map:        strings.TrimSpace(mapName),
		Players:    int(playersByte),
		MaxPlayers: int(maxPlayersByte),
		Ping:       ping,
	}, nil
}

func readNullTerminatedString(r *bytes.Reader) (string, error) {
	var b []byte
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b), nil
}
