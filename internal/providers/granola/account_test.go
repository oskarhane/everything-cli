package granola

import (
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
)

func TestAccountAddWithFlagNeverPrintsKey(t *testing.T) {
	cfg, root, out := newGranolaEnv(t)

	stdout, err := execute(t, root, out, "granola", "account", "add", "work", "--api-key", "grn_secret_flag", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "grn_secret_flag")

	var added map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &added))
	assert.Equal(t, "work", added["name"])

	// Persisted provider-scoped, and the first add becomes the default.
	acct, err := newStore(t, cfg).GetProvider("granola", "work")
	require.NoError(t, err)
	assert.Equal(t, "granola", acct.Provider)
	assert.Contains(t, string(acct.Auth), "grn_secret_flag")
	def, err := newStore(t, cfg).DefaultAccountFor("granola")
	require.NoError(t, err)
	assert.Equal(t, "work", def)
}

func TestAccountAddFromEnv(t *testing.T) {
	cfg, root, out := newGranolaEnv(t)
	t.Setenv("GRANOLA_API_KEY", "grn_secret_env")

	stdout, err := execute(t, root, out, "granola", "account", "add", "personal", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "grn_secret_env")

	acct, err := newStore(t, cfg).GetProvider("granola", "personal")
	require.NoError(t, err)
	assert.Contains(t, string(acct.Auth), "grn_secret_env")
}

func TestAccountAddWithoutKeyFails(t *testing.T) {
	// No flag, no env var, and stdin is not a terminal in tests: capture
	// must fail rather than echo anything.
	_, root, out := newGranolaEnv(t)
	_, err := execute(t, root, out, "granola", "account", "add", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestAccountListMarksDefault(t *testing.T) {
	cfg, root, out := newGranolaEnv(t)
	seedAccount(t, cfg, "alpha", "grn_secret_alpha")
	seedAccount(t, cfg, "beta", "grn_secret_beta")

	stdout, err := execute(t, root, out, "granola", "account", "list", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "grn_secret")

	var accounts []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &accounts))
	require.Len(t, accounts, 2)
	assert.Equal(t, "alpha", accounts[0]["name"])
	assert.Equal(t, true, accounts[0]["default"]) // first add is the default
	assert.Equal(t, "beta", accounts[1]["name"])
	assert.Equal(t, false, accounts[1]["default"])
}

func TestAccountGetNeverPrintsKey(t *testing.T) {
	cfg, root, out := newGranolaEnv(t)
	seedAccount(t, cfg, "work", "grn_secret_get")

	for _, format := range []string{"json", "table", "toon"} {
		stdout, err := execute(t, root, out, "granola", "account", "get", "work", "--format", format)
		require.NoError(t, err, format)
		assert.Contains(t, stdout, "work")
		assert.Contains(t, stdout, "granola")
		assert.NotContains(t, stdout, "grn_secret_get")
	}
}

func TestAccountUseSwitchesDefault(t *testing.T) {
	cfg, root, out := newGranolaEnv(t)
	seedAccount(t, cfg, "alpha", "grn_secret_alpha")
	seedAccount(t, cfg, "beta", "grn_secret_beta")

	_, err := execute(t, root, out, "granola", "account", "use", "beta")
	require.NoError(t, err)
	def, err := newStore(t, cfg).DefaultAccountFor("granola")
	require.NoError(t, err)
	assert.Equal(t, "beta", def)
}

func TestAccountRemoveRequiresForceAndPromotes(t *testing.T) {
	cfg, root, out := newGranolaEnv(t)
	seedAccount(t, cfg, "alpha", "grn_secret_alpha")
	seedAccount(t, cfg, "beta", "grn_secret_beta")

	_, err := execute(t, root, out, "granola", "account", "remove", "alpha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")

	_, err = execute(t, root, out, "granola", "account", "remove", "alpha", "--force")
	require.NoError(t, err)

	// Removing the default promoted the remaining account.
	def, err := newStore(t, cfg).DefaultAccountFor("granola")
	require.NoError(t, err)
	assert.Equal(t, "beta", def)
	_, err = newStore(t, cfg).GetProvider("granola", "alpha")
	require.Error(t, err)
}

func TestResolveAccountFlagAndDefault(t *testing.T) {
	cfg, _, _ := newGranolaEnv(t)
	seedAccount(t, cfg, "alpha", "grn_secret_alpha")
	seedAccount(t, cfg, "beta", "grn_secret_beta")
	store := newStore(t, cfg)

	// Default resolution (first add became the default).
	acct, err := auth.ResolveAccountFor(&app.Config{}, store, providerID)
	require.NoError(t, err)
	assert.Equal(t, "alpha", acct.Name)

	// --account flag wins.
	acct, err = auth.ResolveAccountFor(&app.Config{Account: "beta"}, store, providerID)
	require.NoError(t, err)
	assert.Equal(t, "beta", acct.Name)

	// An unknown flag value fails naming the provider.
	_, err = auth.ResolveAccountFor(&app.Config{Account: "nope"}, store, providerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "granola")
}

func TestResolveAccountProviderNamedErrors(t *testing.T) {
	t.Run("no accounts configured", func(t *testing.T) {
		cfg, _, _ := newGranolaEnv(t)
		_, err := auth.ResolveAccountFor(&app.Config{}, newStore(t, cfg), providerID)
		require.EqualError(t, err, "no granola accounts configured; run `everything-cli granola account add`")
	})

	t.Run("accounts exist but no default set", func(t *testing.T) {
		// Store.Save auto-sets the provider default on first save, so the
		// no-default state can only be constructed by writing the account
		// file directly, with no config.json holding a default.
		cfg, _, _ := newGranolaEnv(t)
		accountJSON := `{"name":"alpha","provider":"granola","auth":{"api_key":"grn_secret_alpha"}}` + "\n"
		require.NoError(t, afero.WriteFile(cfg.Fs, "/config/accounts/granola/alpha.json", []byte(accountJSON), 0o600))

		_, err := auth.ResolveAccountFor(&app.Config{}, newStore(t, cfg), providerID)
		require.EqualError(t, err, "no default granola account set; run `everything-cli granola account use <name>` or pass --account")
	})
}
