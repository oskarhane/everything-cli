package config

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreSaveWritesNestedProviderPath0600 pins the nested layout: Save
// lands the account at accounts/<provider>/<name>.json with owner-only
// permissions. Permission semantics need a real filesystem, so this uses
// OsFs against a throwaway t.TempDir() (the sanctioned exception — it never
// points at the real ~/.config/google-cli).
func TestStoreSaveWritesNestedProviderPath0600(t *testing.T) {
	for _, tt := range []struct {
		name     string
		acct     *Account
		wantPath string
	}{
		{
			name:     "explicit provider",
			acct:     &Account{Name: "work", Provider: "linear", Identity: map[string]string{"email": "me@example.com"}},
			wantPath: "accounts/linear/work.json",
		},
		{
			name:     "empty provider defaults to google",
			acct:     &Account{Name: "work", Email: "user@example.com", Token: testToken("access-1")},
			wantPath: "accounts/google/work.json",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			realFs := afero.NewOsFs()
			root := t.TempDir()
			store, err := NewStore(realFs, root)
			require.NoError(t, err)

			require.NoError(t, store.Save(tt.acct))

			path := filepath.Join(root, tt.wantPath)
			info, err := realFs.Stat(path)
			require.NoError(t, err, "account must persist at the nested provider path")
			assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm(),
				"account file must be owner-only")
		})
	}
}

// TestLegacyFlatAccountLoadsAsGoogle: a pre-provider flat accounts/work.json
// (today's schema, no provider field) loads as a google account and is
// rewritten to accounts/google/work.json on the next Save.
func TestLegacyFlatAccountLoadsAsGoogle(t *testing.T) {
	store := newTestStore(t)
	legacy := `{
  "name": "work",
  "email": "user@example.com",
  "scopes": ["scope-a"],
  "token": {"access_token": "legacy-access", "refresh_token": "legacy-refresh", "token_type": "Bearer"}
}` + "\n"
	require.NoError(t, store.fs.MkdirAll(store.accountsDir(), 0o700))
	require.NoError(t, afero.WriteFile(store.fs, store.legacyAccountPath("work"), []byte(legacy), 0o600))

	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, ProviderGoogle, got.Provider, "a flat file without provider loads as google")
	assert.Equal(t, "work", got.Name)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, []string{"scope-a"}, got.Scopes)
	require.NotNil(t, got.Token)
	assert.Equal(t, "legacy-access", got.Token.AccessToken)

	accounts, err := store.List()
	require.NoError(t, err)
	require.Len(t, accounts, 1, "the legacy flat file must surface in the google list")
	assert.Equal(t, "work", accounts[0].Name)

	require.NoError(t, store.Save(got))

	nested, err := afero.Exists(store.fs, store.AccountPathFor(ProviderGoogle, "work"))
	require.NoError(t, err)
	assert.True(t, nested, "Save must rewrite the account to the nested layout")
	flat, err := afero.Exists(store.fs, store.legacyAccountPath("work"))
	require.NoError(t, err)
	assert.False(t, flat, "Save must remove the migrated legacy flat file")

	reloaded, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, ProviderGoogle, reloaded.Provider)
	assert.Equal(t, "user@example.com", reloaded.Email)

	accounts, err = store.List()
	require.NoError(t, err)
	assert.Len(t, accounts, 1, "migration must not duplicate the account")
}

// TestProviderScopedIsolation: the same account name can exist under two
// providers; each provider's store operations see only its own accounts.
func TestProviderScopedIsolation(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "work", Email: "g@example.com", Token: testToken("g")}))
	require.NoError(t, store.Save(&Account{Name: "work", Provider: "linear", Identity: map[string]string{"email": "l@example.com"}}))

	google, err := store.GetProvider(ProviderGoogle, "work")
	require.NoError(t, err)
	assert.Equal(t, "g@example.com", google.Email)

	linear, err := store.GetProvider("linear", "work")
	require.NoError(t, err)
	assert.Equal(t, ProviderGoogle, google.Provider)
	assert.Equal(t, "linear", linear.Provider)
	assert.Equal(t, "l@example.com", linear.Identity["email"])

	googleAccounts, err := store.List()
	require.NoError(t, err)
	assert.Len(t, googleAccounts, 1, "legacy List sees only google accounts")

	linearAccounts, err := store.ListProvider("linear")
	require.NoError(t, err)
	assert.Len(t, linearAccounts, 1)

	require.NoError(t, store.RemoveProvider("linear", "work"))
	_, err = store.GetProvider("linear", "work")
	assert.Error(t, err)
	_, err = store.GetProvider(ProviderGoogle, "work")
	assert.NoError(t, err, "removing linear's work must leave google's work intact")
}

// TestStoreListAllAggregatesProviders: ListAll is the cross-provider
// aggregate for the read-only top-level account list.
func TestStoreListAllAggregatesProviders(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "home", Email: "g@example.com", Token: testToken("g")}))
	require.NoError(t, store.Save(&Account{Name: "work", Provider: "linear"}))
	require.NoError(t, store.Save(&Account{Name: "acme", Provider: "granola"}))

	all, err := store.ListAll()
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, ProviderGoogle, all[0].Provider)
	assert.Equal(t, "home", all[0].Name)
	assert.Equal(t, "granola", all[1].Provider)
	assert.Equal(t, "linear", all[2].Provider,
		"ListAll sorts by provider, then name")
}

// TestStoreListAllEmptyWithoutAccountsDir mirrors the List behavior for the
// aggregate: no accounts dir is an empty result, not an error.
func TestStoreListAllEmptyWithoutAccountsDir(t *testing.T) {
	store := newTestStore(t)

	all, err := store.ListAll()
	require.NoError(t, err)
	assert.Empty(t, all)
}

// TestStoreListAllIncludesLegacyFlatGoogle: a store holding only a
// pre-provider flat file still aggregates it as a google account.
func TestStoreListAllIncludesLegacyFlatGoogle(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.fs.MkdirAll(store.accountsDir(), 0o700))
	require.NoError(t, afero.WriteFile(store.fs, store.legacyAccountPath("work"),
		[]byte(`{"name":"work","email":"user@example.com"}`+"\n"), 0o600))

	all, err := store.ListAll()
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, ProviderGoogle, all[0].Provider)
	assert.Equal(t, "work", all[0].Name)
}

// TestStoreInvalidProviders: provider IDs become path segments, so the same
// escapes rejected for account names must be rejected here.
func TestStoreInvalidProviders(t *testing.T) {
	store := newTestStore(t)

	for _, table := range []struct {
		name string
		call func() error
	}{
		{"list", func() error { _, err := store.ListProvider(".."); return err }},
		{"get", func() error { _, err := store.GetProvider("a/b", "work"); return err }},
		{"save", func() error {
			return store.Save(&Account{Name: "work", Provider: "a\\b"})
		}},
		{"remove", func() error { return store.RemoveProvider("", "work") }},
		{"set default", func() error { return store.SetDefaultAccountFor(".", "work") }},
	} {
		t.Run(table.name, func(t *testing.T) {
			assert.Error(t, table.call())
		})
	}
}

// TestAccountFileJSONShapeForProvider pins the nested file's schema for a
// non-google account carrying the generic identity/auth shape.
func TestAccountFileJSONShapeForProvider(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{
		Name:     "work",
		Provider: "linear",
		Identity: map[string]string{"email": "me@example.com", "org": "acme"},
		Auth:     json.RawMessage(`{"api_key":"secret-key"}`),
	}))

	data, err := afero.ReadFile(store.fs, store.AccountPathFor("linear", "work"))
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, "work", raw["name"])
	assert.Equal(t, "linear", raw["provider"])
	assert.Equal(t, map[string]any{"email": "me@example.com", "org": "acme"}, raw["identity"])
	assert.Equal(t, map[string]any{"api_key": "secret-key"}, raw["auth"])
}
