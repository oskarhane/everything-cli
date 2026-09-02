package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveTokenSkipsDefaultManagement: the token-refresh persist path must
// never run Save's default management — a provider whose default was
// cleared (e.g. the default account was removed) must still have NO
// default after a refresh persists the new token.
func TestSaveTokenSkipsDefaultManagement(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("old-access")}))
	require.NoError(t, store.clearDefault(ProviderGoogle))

	require.NoError(t, store.SaveToken(ProviderGoogle, "work", testToken("new-access")))

	def, err := store.DefaultAccountFor(ProviderGoogle)
	require.NoError(t, err)
	assert.Empty(t, def, "a token-only write must not re-default the provider")

	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "new-access", got.Token.AccessToken, "the token must be persisted")
	assert.Equal(t, "user@example.com", got.Email, "non-token fields must be preserved")
}

// TestSaveTokenSkipsEmailDedup: SaveToken must not run Save's email dedup,
// which rewrites acct.Name to a same-email sibling's name.
func TestSaveTokenSkipsEmailDedup(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("a")}))
	// Write "alias" directly so Save's dedup does not fold it into "work".
	require.NoError(t, store.fs.MkdirAll(store.providerDir(ProviderGoogle), 0o700))
	require.NoError(t, afero.WriteFile(store.fs, store.AccountPath("alias"),
		[]byte(`{"name":"alias","email":"user@example.com"}`), 0o600))

	require.NoError(t, store.SaveToken(ProviderGoogle, "alias", testToken("b")))

	got, err := store.Get("alias")
	require.NoError(t, err)
	assert.Equal(t, "b", got.Token.AccessToken, "the named record must be updated in place, not deduped away")
}

// TestSaveTokenRequiresExistingAccount: a token-only write for a missing
// account is an error, never an implicit create (which would also have no
// business setting a default).
func TestSaveTokenRequiresExistingAccount(t *testing.T) {
	store := newTestStore(t)

	err := store.SaveToken(ProviderGoogle, "ghost", testToken("a"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
	def, defErr := store.DefaultAccountFor(ProviderGoogle)
	require.NoError(t, defErr)
	assert.Empty(t, def)
}
