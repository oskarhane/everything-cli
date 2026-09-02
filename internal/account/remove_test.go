package account

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoveRequiresForce: without --force the command refuses with an
// error naming --force, and the account survives — for both variants.
func TestRemoveRequiresForce(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		cfg, root, out := newAccountEnv(t, googleSpec)
		seedGoogleAccount(t, cfg, "work", "work@example.com")

		_, err := execute(t, root, out, "account", "remove", "work")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--force", "the error must tell the user to pass --force")
		assert.Contains(t, err.Error(), "work")

		exists, err := afero.Exists(cfg.Fs, newStore(t, cfg).AccountPathFor(googleSpec.ProviderID, "work"))
		require.NoError(t, err)
		assert.True(t, exists, "the token file must survive without --force")
	})

	t.Run("plain", func(t *testing.T) {
		cfg, root, out := newAccountEnv(t, linearSpec)
		seedKeyAccount(t, cfg, linearSpec.ProviderID, "work", "test-key-123")

		_, err := execute(t, root, out, "account", "remove", "work")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--force")

		_, err = newStore(t, cfg).GetProvider(linearSpec.ProviderID, "work")
		require.NoError(t, err, "the account must survive without --force")
	})
}

// TestRemoveDefaultPromotesAndAnnounces: --force removes the account, and
// removing the provider's default promotes another account of that
// provider, announcing the new default — the unified policy for every
// provider.
func TestRemoveDefaultPromotesAndAnnounces(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		cfg, root, out := newAccountEnv(t, googleSpec)
		seedGoogleAccount(t, cfg, "personal", "me@example.com")
		seedGoogleAccount(t, cfg, "work", "work@example.com")
		require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

		outStr, err := execute(t, root, out, "account", "remove", "work", "--force")
		require.NoError(t, err)
		assert.Contains(t, outStr, "removed account work")
		assert.Contains(t, outStr, "default account is now personal",
			"a promotion must be announced")

		exists, err := afero.Exists(cfg.Fs, newStore(t, cfg).AccountPathFor(googleSpec.ProviderID, "work"))
		require.NoError(t, err)
		assert.False(t, exists, "--force must delete the account's token file")

		def, err := newStore(t, cfg).DefaultAccountFor(googleSpec.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, "personal", def, "removing the default must promote another account")
	})

	t.Run("plain", func(t *testing.T) {
		cfg, root, out := newAccountEnv(t, linearSpec)
		seedKeyAccount(t, cfg, linearSpec.ProviderID, "work", "test-key-123")
		seedKeyAccount(t, cfg, linearSpec.ProviderID, "personal", "test-key-456")

		outStr, err := execute(t, root, out, "account", "remove", "work", "--force")
		require.NoError(t, err)
		assert.Contains(t, outStr, "removed account work")
		assert.Contains(t, outStr, "default account is now personal",
			"a promotion must be announced")

		def, err := newStore(t, cfg).DefaultAccountFor(linearSpec.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, "personal", def)
	})
}

// TestRemoveNonDefaultKeepsDefault: removing a non-default account leaves
// the default in place and announces nothing.
func TestRemoveNonDefaultKeepsDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "personal", "me@example.com")
	seedGoogleAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	outStr, err := execute(t, root, out, "account", "remove", "personal", "--force")
	require.NoError(t, err)
	assert.NotContains(t, outStr, "default account is now",
		"no promotion happened, so nothing is announced")

	def, err := newStore(t, cfg).DefaultAccountFor(googleSpec.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, "work", def)
}

// TestRemoveOnlyAccountClearsDefault: removing the last account clears the
// default; with nothing to promote, nothing is announced.
func TestRemoveOnlyAccountClearsDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t, linearSpec)
	seedKeyAccount(t, cfg, linearSpec.ProviderID, "work", "test-key-123")

	outStr, err := execute(t, root, out, "account", "remove", "work", "--force")
	require.NoError(t, err)
	assert.NotContains(t, outStr, "default account is now")

	def, err := newStore(t, cfg).DefaultAccountFor(linearSpec.ProviderID)
	require.NoError(t, err)
	assert.Empty(t, def)
}

// TestRemoveUnknownAccountErrors.
func TestRemoveUnknownAccountErrors(t *testing.T) {
	_, root, out := newAccountEnv(t, googleSpec)

	_, err := execute(t, root, out, "account", "remove", "ghost", "--force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}
