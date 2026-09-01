package account

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoveRequiresForce: without --force the command refuses with an error
// naming --force, and the account's token file survives.
func TestRemoveRequiresForce(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "work", "work@example.com")

	_, err := execute(t, root, out, "account", "remove", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force", "the error must tell the user to pass --force")
	assert.Contains(t, err.Error(), "work")

	exists, err := afero.Exists(cfg.Fs, newStore(t, cfg).AccountPath("work"))
	require.NoError(t, err)
	assert.True(t, exists, "the token file must survive without --force")
}

// TestRemoveWithForceDeletesTokenFile: --force removes the account's token
// file, clears a default that pointed at it, and leaves other accounts.
func TestRemoveWithForceDeletesTokenFile(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "personal", "me@example.com")
	seedAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	outStr, err := execute(t, root, out, "account", "remove", "work", "--force")
	require.NoError(t, err)
	assert.Contains(t, outStr, "work")

	exists, err := afero.Exists(cfg.Fs, newStore(t, cfg).AccountPath("work"))
	require.NoError(t, err)
	assert.False(t, exists, "--force must delete the account's token file")

	def, err := newStore(t, cfg).DefaultAccount()
	require.NoError(t, err)
	assert.Empty(t, def, "removing the default account must clear the default")

	accounts, err := newStore(t, cfg).List()
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "personal", accounts[0].Name)
}

// TestRemoveWithForceKeepsOtherDefault: removing a non-default account
// leaves the default in place.
func TestRemoveWithForceKeepsOtherDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "personal", "me@example.com")
	seedAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	_, err := execute(t, root, out, "account", "remove", "personal", "--force")
	require.NoError(t, err)

	def, err := newStore(t, cfg).DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "work", def)
}

// TestRemoveUnknownAccountErrors.
func TestRemoveUnknownAccountErrors(t *testing.T) {
	_, root, out := newAccountEnv(t)

	_, err := execute(t, root, out, "account", "remove", "ghost", "--force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}
