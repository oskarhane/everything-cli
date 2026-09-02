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
