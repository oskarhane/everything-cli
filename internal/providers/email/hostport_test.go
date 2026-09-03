package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitHostPort pins the shared host:port parsing used by both the
// account-add normalization and the dial path: bare hosts take the
// default, embedded ports win, IPv6 literals work both bare and
// bracketed, and malformed values are clear errors.
func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		defaultPort int
		wantHost    string
		wantPort    int
		wantErr     string
	}{
		{name: "bare hostname", value: "imap.example.com", defaultPort: 993,
			wantHost: "imap.example.com", wantPort: 993},
		{name: "bare ipv4", value: "127.0.0.1", defaultPort: 993,
			wantHost: "127.0.0.1", wantPort: 993},
		{name: "hostname with port", value: "imap.example.com:1143", defaultPort: 993,
			wantHost: "imap.example.com", wantPort: 1143},
		{name: "ipv4 with port", value: "127.0.0.1:1143", defaultPort: 993,
			wantHost: "127.0.0.1", wantPort: 1143},
		{name: "bracketed ipv6 with port", value: "[::1]:1143", defaultPort: 993,
			wantHost: "::1", wantPort: 1143},
		{name: "bare ipv6", value: "::1", defaultPort: 993,
			wantHost: "::1", wantPort: 993},
		{name: "bracketed bare ipv6", value: "[::1]", defaultPort: 993,
			wantHost: "::1", wantPort: 993},
		{name: "empty host", value: ":1143", defaultPort: 993,
			wantErr: `no host before the port`},
		{name: "garbage port", value: "h:notaport", defaultPort: 993,
			wantErr: `port "notaport" is not a number`},
		{name: "out of range port", value: "h:99999", defaultPort: 993,
			wantErr: "port must be between 1 and 65535"},
		{name: "zero port", value: "h:0", defaultPort: 993,
			wantErr: "port must be between 1 and 65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := splitHostPort(tt.value, tt.defaultPort)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}

// TestResolveDialServer pins the dial-time precedence: a port embedded in
// the stored host wins over the stored port field (legacy accounts saved
// before add-time normalization), while a pure stored host keeps the
// stored port.
func TestResolveDialServer(t *testing.T) {
	tests := []struct {
		name     string
		srv      serverConfig
		wantHost string
		wantPort int
	}{
		{name: "legacy embedded port wins over stored default",
			srv:      serverConfig{Host: "127.0.0.1:1143", Port: defaultIMAPPort},
			wantHost: "127.0.0.1", wantPort: 1143},
		{name: "normalized pure host keeps stored port",
			srv:      serverConfig{Host: "imap.example.com", Port: 1143},
			wantHost: "imap.example.com", wantPort: 1143},
		{name: "zero stored port falls back to the default",
			srv:      serverConfig{Host: "imap.example.com"},
			wantHost: "imap.example.com", wantPort: defaultIMAPPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := resolveDialServer(tt.srv, defaultIMAPPort)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}
