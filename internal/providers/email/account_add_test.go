package email

import (
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountAddWithFlagsNeverPrintsPassword pins the add contract: the
// command persists the full credential set provider-scoped at 0600 and
// prints the account name only.
func TestAccountAddWithFlagsNeverPrintsPassword(t *testing.T) {
	cfg, root, out := newEmailEnv(t)

	stdout, err := execute(t, root, out,
		"email", "account", "add", "work",
		"--imap-host", "imap.example.com", "--smtp-host", "smtp.example.com",
		"--username", "me@example.com", "--password", "secret-add-flag",
		"--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "secret-add-flag")

	var added map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &added))
	assert.Equal(t, map[string]any{"name": "work"}, added,
		"add prints the name only — never credentials")

	// Persisted provider-scoped at 0600 with the full auth payload.
	info, err := cfg.Fs.Stat("/config/accounts/email/work.json")
	require.NoError(t, err)
	assert.Equal(t, 0o600, int(info.Mode().Perm()))

	var saved struct {
		Auth credentials `json:"auth"`
	}
	raw, err := afero.ReadFile(cfg.Fs, "/config/accounts/email/work.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &saved))
	payload := saved.Auth
	assert.Equal(t, "me@example.com", payload.Username)
	assert.Equal(t, "secret-add-flag", payload.Password)
	assert.Equal(t, "imap.example.com", payload.IMAP.Host)
	assert.Equal(t, 993, payload.IMAP.Port, "default IMAP port")
	assert.Equal(t, "smtp.example.com", payload.SMTP.Host)
	assert.Equal(t, 587, payload.SMTP.Port, "default SMTP port")

	// The first add becomes the provider default.
	def, err := newStore(t, cfg).DefaultAccountFor("email")
	require.NoError(t, err)
	assert.Equal(t, "work", def)
}

func TestAccountAddCustomPorts(t *testing.T) {
	cfg, root, out := newEmailEnv(t)

	_, err := execute(t, root, out,
		"email", "account", "add", "work",
		"--imap-host", "imap.example.com", "--imap-port", "143",
		"--smtp-host", "smtp.example.com", "--smtp-port", "465",
		"--username", "me@example.com", "--password", "secret-ports")
	require.NoError(t, err)

	acct, err := newStore(t, cfg).GetProvider("email", "work")
	require.NoError(t, err)
	creds, err := loadCredentials(acct)
	require.NoError(t, err)
	assert.Equal(t, 143, creds.IMAP.Port)
	assert.Equal(t, 465, creds.SMTP.Port)
}

// TestAccountAddHostPortSyntax proves the bug fix: a port embedded in
// --imap-host/--smtp-host is split off at add time, so the stored payload
// keeps a pure host and the resolved integer port.
func TestAccountAddHostPortSyntax(t *testing.T) {
	cfg, root, out := newEmailEnv(t)

	_, err := execute(t, root, out,
		"email", "account", "add", "personal",
		"--imap-host", "127.0.0.1:1143", "--smtp-host", "127.0.0.1:1025",
		"--username", "u", "--password", "secret-host-port")
	require.NoError(t, err)

	acct, err := newStore(t, cfg).GetProvider("email", "personal")
	require.NoError(t, err)
	creds, err := loadCredentials(acct)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", creds.IMAP.Host)
	assert.Equal(t, 1143, creds.IMAP.Port)
	assert.Equal(t, "127.0.0.1", creds.SMTP.Host)
	assert.Equal(t, 1025, creds.SMTP.Port)
}

// TestAccountAddPortPrecedence pins the port precedence: an explicit
// --imap-port/--smtp-port beats a port embedded in the host value, and an
// embedded port beats the defaults.
func TestAccountAddPortPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		extraArgs    []string
		wantIMAPHost string
		wantIMAPPort int
		wantSMTPHost string
		wantSMTPPort int
	}{
		{name: "explicit port flag overrides embedded host port",
			extraArgs: []string{"--imap-host", "imap.example.com:1143", "--imap-port", "1993",
				"--smtp-host", "smtp.example.com:1025", "--smtp-port", "1465"},
			wantIMAPHost: "imap.example.com", wantIMAPPort: 1993,
			wantSMTPHost: "smtp.example.com", wantSMTPPort: 1465},
		{name: "embedded host port overrides the defaults",
			extraArgs:    []string{"--imap-host", "[::1]:1143", "--smtp-host", "smtp.example.com:1025"},
			wantIMAPHost: "::1", wantIMAPPort: 1143,
			wantSMTPHost: "smtp.example.com", wantSMTPPort: 1025},
		{name: "bare host keeps the defaults",
			extraArgs:    []string{"--imap-host", "imap.example.com", "--smtp-host", "smtp.example.com"},
			wantIMAPHost: "imap.example.com", wantIMAPPort: 993,
			wantSMTPHost: "smtp.example.com", wantSMTPPort: 587},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, root, out := newEmailEnv(t)
			args := append([]string{"email", "account", "add", "work",
				"--username", "me@example.com", "--password", "secret-precedence"}, tt.extraArgs...)
			_, err := execute(t, root, out, args...)
			require.NoError(t, err)

			acct, err := newStore(t, cfg).GetProvider("email", "work")
			require.NoError(t, err)
			creds, err := loadCredentials(acct)
			require.NoError(t, err)
			assert.Equal(t, tt.wantIMAPHost, creds.IMAP.Host)
			assert.Equal(t, tt.wantIMAPPort, creds.IMAP.Port)
			assert.Equal(t, tt.wantSMTPHost, creds.SMTP.Host)
			assert.Equal(t, tt.wantSMTPPort, creds.SMTP.Port)
		})
	}
}

// TestAccountAddInvalidHostPort rejects malformed host:port values with
// clear usage errors before any password is captured.
func TestAccountAddInvalidHostPort(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "garbage port",
			args:    []string{"--imap-host", "h:notaport", "--smtp-host", "smtp.example.com"},
			wantErr: `--imap-host invalid host "h:notaport": port "notaport" is not a number`},
		{name: "empty host",
			args:    []string{"--imap-host", ":1143", "--smtp-host", "smtp.example.com"},
			wantErr: "no host before the port"},
		{name: "out of range port",
			args:    []string{"--imap-host", "imap.example.com", "--smtp-host", "h:99999"},
			wantErr: `--smtp-host invalid host "h:99999": port must be between 1 and 65535`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root, out := newEmailEnv(t)
			args := append([]string{"email", "account", "add", "work",
				"--username", "me@example.com", "--password", "secret-invalid-host"}, tt.args...)
			_, err := execute(t, root, out, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAccountAddFromEnv(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	t.Setenv("EMAIL_PASSWORD", "secret-add-env")

	stdout, err := execute(t, root, out,
		"email", "account", "add", "personal",
		"--imap-host", "imap.example.com", "--smtp-host", "smtp.example.com",
		"--username", "me@example.com", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "secret-add-env")

	acct, err := newStore(t, cfg).GetProvider("email", "personal")
	require.NoError(t, err)
	assert.Contains(t, string(acct.Auth), "secret-add-env")
}

func TestAccountAddRequiresUsername(t *testing.T) {
	_, root, out := newEmailEnv(t)

	_, err := execute(t, root, out,
		"email", "account", "add", "work",
		"--imap-host", "imap.example.com", "--smtp-host", "smtp.example.com",
		"--password", "secret-no-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--username")
}

func TestAccountAddRequiresHosts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"missing imap host",
			[]string{"--smtp-host", "smtp.example.com"}, "--imap-host"},
		{"missing smtp host",
			[]string{"--imap-host", "imap.example.com"}, "--smtp-host"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, root, out := newEmailEnv(t)
			args := append([]string{"email", "account", "add", "work",
				"--username", "me@example.com", "--password", "secret-no-host"}, tt.args...)
			_, err := execute(t, root, out, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAccountAddWithoutPasswordFails(t *testing.T) {
	// No flag, no env var, and stdin is not a terminal in tests: capture
	// must fail rather than echo anything.
	_, root, out := newEmailEnv(t)
	_, err := execute(t, root, out,
		"email", "account", "add", "work",
		"--imap-host", "imap.example.com", "--smtp-host", "smtp.example.com",
		"--username", "me@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}
