package slack

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountAddValidatesAndStoresIdentity: add runs the real validate hook
// against an httptest auth.test, persists the identity map in a 0600
// provider-scoped file, and never prints the token in any output format.
func TestAccountAddValidatesAndStoresIdentity(t *testing.T) {
	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			cfg, root, out := newSlackEnv(t)
			stubAuthTest(t, authTestOK)
			const token = "xoxp-secret-add"

			stdout, err := execute(t, root, out, "slack", "account", "add", "work",
				"--api-key", token, "--format", format)
			require.NoError(t, err)
			assert.Contains(t, stdout, "work")
			assert.NotContains(t, stdout, token, "format %s leaked the token", format)

			path := "/config/accounts/slack/work.json"
			raw, err := afero.ReadFile(cfg.Fs, path)
			require.NoError(t, err, "the account lands nested per provider")
			info, err := cfg.Fs.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "account files are private")
			assert.Contains(t, string(raw), token, "the token is stored, just never printed")

			var saved struct {
				Name     string            `json:"name"`
				Provider string            `json:"provider"`
				Identity map[string]string `json:"identity"`
			}
			require.NoError(t, json.Unmarshal(raw, &saved))
			assert.Equal(t, "work", saved.Name)
			assert.Equal(t, "slack", saved.Provider)
			assert.Equal(t, map[string]string{
				"team":    "Neo4j",
				"team_id": "T0B5K0M1AAC",
				"user":    "oskar",
				"user_id": "U02H6ECK2",
			}, saved.Identity)

			// The first add becomes the provider default.
			def, err := newStore(t, cfg).DefaultAccountFor(providerID)
			require.NoError(t, err)
			assert.Equal(t, "work", def)
		})
	}
}

// TestAccountAddFromEnv: with no flag value the token comes from
// SLACK_API_KEY and the same validation and persistence path runs.
func TestAccountAddFromEnv(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	stubAuthTest(t, authTestOK)
	const token = "xoxp-secret-env"
	t.Setenv("SLACK_API_KEY", token)

	stdout, err := execute(t, root, out, "slack", "account", "add", "personal", "--format", "json")
	require.NoError(t, err)
	assert.Contains(t, stdout, "personal")
	assert.NotContains(t, stdout, token)

	acct, err := newStore(t, cfg).GetProvider(providerID, "personal")
	require.NoError(t, err)
	assert.Equal(t, "oskar", acct.Identity["user"])
}

// TestAccountAddWithoutTokenFails: no flag, no env var, and stdin is not a
// terminal in tests: capture fails rather than echo anything.
func TestAccountAddWithoutTokenFails(t *testing.T) {
	_, root, out := newSlackEnv(t)
	_, err := execute(t, root, out, "slack", "account", "add", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

// TestAccountAddRejectedTokenWritesNothing: an auth.test ok:false response
// aborts add before persistence; the Slack error code is visible and no
// account file is left behind.
func TestAccountAddRejectedTokenWritesNothing(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	stubAuthTest(t, `{"ok":false,"error":"invalid_auth"}`)

	_, err := execute(t, root, out, "slack", "account", "add", "work",
		"--api-key", "xoxp-rejected", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_auth")
	assert.NotContains(t, err.Error(), "xoxp-rejected", "the error must not carry the token")

	files, globErr := afero.Glob(cfg.Fs, "/config/accounts/slack/*.json")
	require.NoError(t, globErr)
	assert.Empty(t, files, "a rejected token must not create an account file")
}

// TestAccountAddWarnsOnBotToken: an xoxb token cannot call search.messages,
// so add warns on the configured stderr writer (mentioning xoxp) but still
// stores the account.
func TestAccountAddWarnsOnBotToken(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	stubAuthTest(t, `{"ok":true,"team":"Neo4j","team_id":"T0B5K0M1AAC","user":"botskar","user_id":"U0BOT"}`)
	warned := stubWarnWriter(t)
	const token = "xoxb-bot-token"

	stdout, err := execute(t, root, out, "slack", "account", "add", "bot",
		"--api-key", token, "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, token)
	assert.Contains(t, warned.String(), "xoxp", "a non-user token must warn that search needs an xoxp token")

	acct, err := newStore(t, cfg).GetProvider(providerID, "bot")
	require.NoError(t, err, "the warning is non-fatal; the account is stored")
	assert.Equal(t, "U0BOT", acct.Identity["user_id"])
}

// TestAccountListMarksDefault: the shared list leaf runs under slack and
// marks the first-added account as the default, without printing tokens.
func TestAccountListMarksDefault(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	seedAccount(t, cfg, "alpha", "xoxp-secret-alpha")
	seedAccount(t, cfg, "beta", "xoxp-secret-beta")

	stdout, err := execute(t, root, out, "slack", "account", "list", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "xoxp-secret")

	var accounts []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &accounts))
	require.Len(t, accounts, 2)
	assert.Equal(t, "alpha", accounts[0]["name"])
	assert.Equal(t, true, accounts[0]["default"])
	assert.Equal(t, "beta", accounts[1]["name"])
	assert.Equal(t, false, accounts[1]["default"])
}

// TestAccountGetShowsIdentityNeverToken: the identity-aware shared get
// renders the auth.test metadata as top-level fields in every format and
// never the stored token.
func TestAccountGetShowsIdentityNeverToken(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	seedAccount(t, cfg, "work", "xoxp-secret-get")

	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			stdout, err := execute(t, root, out, "slack", "account", "get", "work", "--format", format)
			require.NoError(t, err)
			assert.Contains(t, stdout, "work")
			assert.Contains(t, stdout, "slack")
			assert.Contains(t, stdout, "seed-user")
			assert.Contains(t, stdout, "Seed Team")
			assert.NotContains(t, stdout, "xoxp-secret-get", "format %s leaked the token", format)
		})
	}
}

// TestAccountGetTableHeadersIncludeIdentity: go-pretty upper-cases header
// cells; the identity keys join name and provider as dynamic columns.
func TestAccountGetTableHeadersIncludeIdentity(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	seedAccount(t, cfg, "work", "xoxp-secret-headers")

	stdout, err := execute(t, root, out, "slack", "account", "get", "work", "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"NAME", "PROVIDER", "TEAM", "TEAM_ID", "USER", "USER_ID"} {
		assert.Contains(t, stdout, header)
	}
}

// TestAccountRemoveRequiresForceAndPromotes: the shared remove leaf guards
// with --force and promotes the remaining account when the default goes.
func TestAccountRemoveRequiresForceAndPromotes(t *testing.T) {
	cfg, root, out := newSlackEnv(t)
	seedAccount(t, cfg, "alpha", "xoxp-secret-alpha")
	seedAccount(t, cfg, "beta", "xoxp-secret-beta")

	_, err := execute(t, root, out, "slack", "account", "remove", "alpha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")

	stdout, err := execute(t, root, out, "slack", "account", "remove", "alpha", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "removed account alpha")
	assert.Contains(t, stdout, "default account is now beta")

	def, err := newStore(t, cfg).DefaultAccountFor(providerID)
	require.NoError(t, err)
	assert.Equal(t, "beta", def)
	_, err = newStore(t, cfg).GetProvider(providerID, "alpha")
	require.Error(t, err)
}
