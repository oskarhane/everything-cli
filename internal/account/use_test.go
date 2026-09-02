package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUseSetsDefault: account use persists the provider's default account,
// for both variants.
func TestUseSetsDefault(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		cfg, root, out := newAccountEnv(t, googleSpec)
		seedGoogleAccount(t, cfg, "personal", "me@example.com")
		seedGoogleAccount(t, cfg, "work", "work@example.com")

		outStr, err := execute(t, root, out, "account", "use", "work")
		require.NoError(t, err)
		assert.Contains(t, outStr, "default account set to work")

		def, err := newStore(t, cfg).DefaultAccountFor(googleSpec.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, "work", def)
	})

	t.Run("plain", func(t *testing.T) {
		cfg, root, out := newAccountEnv(t, linearSpec)
		seedKeyAccount(t, cfg, linearSpec.ProviderID, "work", "test-key-123")
		seedKeyAccount(t, cfg, linearSpec.ProviderID, "personal", "test-key-456")

		outStr, err := execute(t, root, out, "account", "use", "personal")
		require.NoError(t, err)
		assert.Contains(t, outStr, "default account set to personal")

		def, err := newStore(t, cfg).DefaultAccountFor(linearSpec.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, "personal", def)
	})
}

// TestUseUnknownAccountErrors: using an unknown name errors and leaves the
// default unchanged.
func TestUseUnknownAccountErrors(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	_, err := execute(t, root, out, "account", "use", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
	assert.Contains(t, err.Error(), "setting default account")

	def, err := newStore(t, cfg).DefaultAccountFor(googleSpec.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, "work", def, "the default must survive a failed use")
}
