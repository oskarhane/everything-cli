package email

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountListMarksDefault(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	seedAccount(t, cfg, "alpha", "pw_secret_alpha")
	seedAccount(t, cfg, "beta", "pw_secret_beta")

	stdout, err := execute(t, root, out, "email", "account", "list", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "pw_secret")

	var accounts []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &accounts))
	require.Len(t, accounts, 2)
	assert.Equal(t, "alpha", accounts[0]["name"])
	assert.Equal(t, true, accounts[0]["default"]) // first add is the default
	assert.Equal(t, "beta", accounts[1]["name"])
	assert.Equal(t, false, accounts[1]["default"])
}

func TestAccountGetNeverPrintsPassword(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	seedAccount(t, cfg, "work", "pw_secret_get")

	for _, format := range []string{"json", "table", "toon"} {
		stdout, err := execute(t, root, out, "email", "account", "get", "work", "--format", format)
		require.NoError(t, err, format)
		assert.Contains(t, stdout, "work")
		assert.Contains(t, stdout, "email")
		assert.NotContains(t, stdout, "pw_secret_get")
	}
}

func TestAccountUseSetsDefault(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	seedAccount(t, cfg, "alpha", "pw_secret_alpha")
	seedAccount(t, cfg, "beta", "pw_secret_beta")

	stdout, err := execute(t, root, out, "email", "account", "use", "beta")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "pw_secret")

	def, err := newStore(t, cfg).DefaultAccountFor("email")
	require.NoError(t, err)
	assert.Equal(t, "beta", def)
}

func TestAccountRemoveRequiresForceAndPromotes(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	seedAccount(t, cfg, "alpha", "pw_secret_alpha")
	seedAccount(t, cfg, "beta", "pw_secret_beta")

	_, err := execute(t, root, out, "email", "account", "remove", "alpha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")

	stdout, err := execute(t, root, out, "email", "account", "remove", "alpha", "--force")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "pw_secret")
	assert.Contains(t, stdout, "removed account alpha")
	assert.Contains(t, stdout, "default account is now beta",
		"removing the default must announce the promoted account")

	// Removing the default promoted the remaining account.
	def, err := newStore(t, cfg).DefaultAccountFor("email")
	require.NoError(t, err)
	assert.Equal(t, "beta", def)
	_, err = newStore(t, cfg).GetProvider("email", "alpha")
	require.Error(t, err)
}
