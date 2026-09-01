package config

import (
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveSetsProviderDefaultWhenNone: the first account saved for a
// provider becomes its default; later saves never override an existing one.
func TestSaveSetsProviderDefaultWhenNone(t *testing.T) {
	t.Run("google", func(t *testing.T) {
		store := newTestStore(t)
		require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("a")}))

		def, err := store.DefaultAccount()
		require.NoError(t, err)
		assert.Equal(t, "work", def, "first google save must set the google default")

		require.NoError(t, store.Save(&Account{Name: "alt", Email: "alt@example.com", Token: testToken("b")}))
		def, err = store.DefaultAccount()
		require.NoError(t, err)
		assert.Equal(t, "work", def, "a later save must not override the existing default")
	})

	t.Run("other provider", func(t *testing.T) {
		store := newTestStore(t)
		require.NoError(t, store.Save(&Account{Name: "work", Provider: "linear"}))

		def, err := store.DefaultAccountFor("linear")
		require.NoError(t, err)
		assert.Equal(t, "work", def)

		def, err = store.DefaultAccount()
		require.NoError(t, err)
		assert.Empty(t, def, "a linear default must not leak into google")
	})
}

// TestRemoveProviderPromotesDefault: removing a provider's default promotes
// another of its accounts; removing the last one clears the default.
func TestRemoveProviderPromotesDefault(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "beta", Provider: "linear"}))
	require.NoError(t, store.Save(&Account{Name: "alpha", Provider: "linear"}))

	def, err := store.DefaultAccountFor("linear")
	require.NoError(t, err)
	assert.Equal(t, "beta", def, "first save becomes the default")

	require.NoError(t, store.RemoveProvider("linear", "beta"))
	def, err = store.DefaultAccountFor("linear")
	require.NoError(t, err)
	assert.Equal(t, "alpha", def, "removing the default must promote the remaining account")

	require.NoError(t, store.RemoveProvider("linear", "alpha"))
	def, err = store.DefaultAccountFor("linear")
	require.NoError(t, err)
	assert.Empty(t, def, "removing the last account must clear the provider default")
}

// TestRemoveProviderPromotionIsPerProvider: promoting a default never
// crosses provider boundaries.
func TestRemoveProviderPromotionIsPerProvider(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "work", Provider: "linear"}))
	require.NoError(t, store.Save(&Account{Name: "home", Email: "g@example.com", Token: testToken("g")}))

	require.NoError(t, store.RemoveProvider("linear", "work"))

	def, err := store.DefaultAccountFor("linear")
	require.NoError(t, err)
	assert.Empty(t, def, "no linear account remains — the default clears, never borrows google's")
	def, err = store.DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "home", def, "google's default is untouched")
}

// TestLegacyRemoveClearsDefaultWithoutPromotion pins the transitional
// compat policy: the legacy Remove keeps the pre-provider behavior (clear,
// never promote) so existing callers behave unchanged until the command
// tree adopts RemoveProvider.
func TestLegacyRemoveClearsDefaultWithoutPromotion(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "personal", Email: "me@example.com", Token: testToken("a")}))
	require.NoError(t, store.Save(&Account{Name: "work", Email: "work@example.com", Token: testToken("b")}))
	require.NoError(t, store.SetDefaultAccount("work"))

	require.NoError(t, store.Remove("work"))

	def, err := store.DefaultAccount()
	require.NoError(t, err)
	assert.Empty(t, def, "legacy Remove clears the default, exactly as before providers")
}

// TestLegacyDefaultAccountMigratesOnRead: a legacy config.json holding
// {"default_account": "work"} reads as default_accounts.google, and the
// next settings write persists the migrated shape.
func TestLegacyDefaultAccountMigratesOnRead(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.fs.MkdirAll(store.root, 0o700))
	require.NoError(t, afero.WriteFile(store.fs, store.settingsPath(),
		[]byte(`{"default_account": "work"}`+"\n"), 0o600))

	def, err := store.DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "work", def, "legacy default_account reads as the google default")

	def, err = store.DefaultAccountFor(ProviderGoogle)
	require.NoError(t, err)
	assert.Equal(t, "work", def)

	// The migration must not auto-set a default on first save: the legacy
	// default already names one.
	require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("a")}))

	// The next settings write persists the migrated shape.
	require.NoError(t, store.SetDefaultAccount("work"))
	data, err := afero.ReadFile(store.fs, store.settingsPath())
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	_, hasLegacy := raw["default_account"]
	assert.False(t, hasLegacy, "the legacy key is dropped once settings are written")
	assert.Equal(t, map[string]any{"google": "work"}, raw["default_accounts"])
}

// TestSetDefaultAccountForPreservesOtherProviders: per-provider defaults
// share config.json; setting one must not clobber the others.
func TestSetDefaultAccountForPreservesOtherProviders(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "home", Email: "g@example.com", Token: testToken("g")}))
	require.NoError(t, store.Save(&Account{Name: "work", Provider: "linear"}))

	require.NoError(t, store.SetDefaultAccountFor("linear", "work"))
	require.NoError(t, store.SetDefaultAccount("home"))

	def, err := store.DefaultAccountFor("linear")
	require.NoError(t, err)
	assert.Equal(t, "work", def, "setting the google default must preserve linear's")
	def, err = store.DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "home", def)
}

// TestSetDefaultAccountForRequiresExistingAccount, per provider.
func TestSetDefaultAccountForRequiresExistingAccount(t *testing.T) {
	store := newTestStore(t)

	err := store.SetDefaultAccountFor("linear", "ghost")
	assert.Error(t, err, "a default must reference an existing account of that provider")
}
