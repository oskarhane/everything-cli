package email

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

var errPromptBroke = errors.New("prompt broke")

func testAddOptions(name, password string) addOptions {
	return addOptions{
		Name:     name,
		Username: "me@example.com",
		Password: password,
		IMAPHost: "imap.example.com",
		IMAPPort: defaultIMAPPort,
		SMTPHost: "smtp.example.com",
		SMTPPort: defaultSMTPPort,
	}
}

// TestAddCapturesFromFlag: a flag-provided password is stored
// provider-scoped inside Account.Auth, and env/prompt are never consulted.
func TestAddCapturesFromFlag(t *testing.T) {
	cfg, _, _ := newEmailEnv(t)
	stubCapture(t,
		func(string) string {
			t.Fatal("getenv must not be consulted when the flag supplies the password")
			return ""
		},
		func() (string, error) {
			t.Fatal("prompt must not be consulted when the flag supplies the password")
			return "", nil
		})

	acct, err := addAccount(newStore(t, cfg), testAddOptions("work", "secret-flag"))
	require.NoError(t, err)
	assert.Equal(t, "work", acct.Name)
	assert.Equal(t, "email", acct.Provider)
	assert.JSONEq(t, `{
		"username": "me@example.com",
		"password": "secret-flag",
		"imap": {"host": "imap.example.com", "port": 993},
		"smtp": {"host": "smtp.example.com", "port": 587}
	}`, string(acct.Auth))

	raw, err := afero.ReadFile(cfg.Fs, "/config/accounts/email/work.json")
	require.NoError(t, err, "the account lands nested per provider")
	assert.Contains(t, string(raw), `"password"`)
}

// TestAddCapturesFromEnv: with no flag value, EMAIL_PASSWORD supplies the
// password and the prompt stays untouched.
func TestAddCapturesFromEnv(t *testing.T) {
	cfg, _, _ := newEmailEnv(t)
	stubCapture(t,
		func(name string) string {
			assert.Equal(t, "EMAIL_PASSWORD", name)
			return "secret-env"
		},
		func() (string, error) {
			t.Fatal("prompt must not be consulted when the env var supplies the password")
			return "", nil
		})

	acct, err := addAccount(newStore(t, cfg), testAddOptions("personal", ""))
	require.NoError(t, err)
	assert.Contains(t, string(acct.Auth), "secret-env")
}

// TestAddCapturesFromHiddenPrompt: with neither flag nor env, the hidden
// prompt supplies the password.
func TestAddCapturesFromHiddenPrompt(t *testing.T) {
	cfg, _, _ := newEmailEnv(t)
	stubCapture(t,
		func(string) string { return "" },
		func() (string, error) { return "secret-prompt", nil })

	acct, err := addAccount(newStore(t, cfg), testAddOptions("main", ""))
	require.NoError(t, err)
	assert.Contains(t, string(acct.Auth), "secret-prompt")
}

func TestAddNoPasswordAvailable(t *testing.T) {
	t.Run("empty everywhere names the env var", func(t *testing.T) {
		cfg, _, _ := newEmailEnv(t)
		stubCapture(t,
			func(string) string { return "" },
			func() (string, error) { return "  ", nil })
		_, err := addAccount(newStore(t, cfg), testAddOptions("work", ""))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "EMAIL_PASSWORD",
			"the error names the env var, never a password value")
	})
	t.Run("prompt failure propagates", func(t *testing.T) {
		cfg, _, _ := newEmailEnv(t)
		stubCapture(t,
			func(string) string { return "" },
			func() (string, error) { return "", errPromptBroke })
		_, err := addAccount(newStore(t, cfg), testAddOptions("work", ""))
		assert.ErrorIs(t, err, errPromptBroke)
	})
}

func TestAddValidatesEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(*addOptions)
		wantErr string
	}{
		{"missing username", func(o *addOptions) { o.Username = " " }, "--username"},
		{"missing imap host", func(o *addOptions) { o.IMAPHost = "" }, "--imap-host"},
		{"missing smtp host", func(o *addOptions) { o.SMTPHost = "" }, "--smtp-host"},
		{"imap port out of range", func(o *addOptions) { o.IMAPPort = 0 }, "--imap-port"},
		{"smtp port out of range", func(o *addOptions) { o.SMTPPort = 70000 }, "--smtp-port"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _, _ := newEmailEnv(t)
			stubCapture(t, nil, func() (string, error) {
				t.Fatal("prompt must not be reached when validation fails first")
				return "", nil
			})
			opts := testAddOptions("work", "secret-validation")
			tt.mutate(&opts)
			_, err := addAccount(newStore(t, cfg), opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoadCredentialsRejectsUnusableAccount(t *testing.T) {
	_, err := loadCredentials(nil)
	assert.Error(t, err, "nil account")
	_, err = loadCredentials(&config.Account{Name: "work", Auth: json.RawMessage(`{`)})
	assert.Error(t, err, "malformed auth payload")
	_, err = loadCredentials(&config.Account{Name: "work", Auth: json.RawMessage(`{}`)})
	assert.Error(t, err, "payload without a password")
}

// TestLoadCredentialsParsesStoredPayload: loadCredentials hands the mail
// adapter (later work) the full credential set parsed from the account's
// auth payload.
func TestLoadCredentialsParsesStoredPayload(t *testing.T) {
	cfg, _, _ := newEmailEnv(t)
	_, err := addAccount(newStore(t, cfg), testAddOptions("work", "secret-load"))
	require.NoError(t, err)

	acct, err := newStore(t, cfg).GetProvider("email", "work")
	require.NoError(t, err)
	creds, err := loadCredentials(acct)
	require.NoError(t, err)
	assert.Equal(t, "me@example.com", creds.Username)
	assert.Equal(t, "secret-load", creds.Password)
	assert.Equal(t, serverConfig{Host: "imap.example.com", Port: 993}, creds.IMAP)
	assert.Equal(t, serverConfig{Host: "smtp.example.com", Port: 587}, creds.SMTP)
}

// TestPasswordRegisteredForRedaction pins the AGENTS.md mint/read-point
// rule: after addAccount captures a password (and after loadCredentials
// reads it back from disk), the redactor scrubs that password from any
// output — so an account get rendering that passes through Redact can
// never leak it.
func TestPasswordRegisteredForRedaction(t *testing.T) {
	cfg, _, _ := newEmailEnv(t)
	stubCapture(t, func(string) string { return "" }, nil)
	// Inside the prompt the password exists but capture has not happened
	// yet; nothing has had a chance to print it.
	stubCapture(t, nil, func() (string, error) {
		assert.NotContains(t, auth.Redact("secret-redact"), "***",
			"registration happens at capture, not before the password exists")
		return "secret-redact", nil
	})

	_, err := addAccount(newStore(t, cfg), testAddOptions("work", ""))
	require.NoError(t, err)

	// After capture, any output string carrying the password is scrubbed.
	out := auth.Redact("name: work\nauth:\n  password: secret-redact\n")
	assert.NotContains(t, out, "secret-redact")
	assert.Contains(t, out, "***")

	// The persisted account document — what a generic account get would
	// render — is safe once redacted.
	raw, err := afero.ReadFile(cfg.Fs, "/config/accounts/email/work.json")
	require.NoError(t, err)
	scrubbed := auth.Redact(string(raw))
	assert.NotContains(t, scrubbed, "secret-redact")

	// After loadCredentials re-reads the password from disk (a fresh
	// process in real use), the redactor still scrubs it.
	_, err = loadCredentials(&config.Account{
		Name: "work", Provider: "email", Auth: json.RawMessage(`{"username":"u","password":"secret-load-point"}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, auth.Redact("leak: secret-load-point"), "secret-load-point")
}
