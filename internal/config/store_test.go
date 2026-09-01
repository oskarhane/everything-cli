package config

import (
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreSaveWrites0600 asserts the token cache permission against a real
// filesystem (in a temp dir), since permission semantics are what matter.
func TestStoreSaveWrites0600(t *testing.T) {
	realFs := afero.NewOsFs()
	store, err := NewStore(realFs, t.TempDir())
	require.NoError(t, err)

	err = store.Save(&Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  testToken("access-1"),
	})
	require.NoError(t, err)

	info, err := realFs.Stat(store.AccountPath("work"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm(), "account file must be owner-only")
}

func TestStoreSaveGetRoundTrip(t *testing.T) {
	store := newTestStore(t)
	want := &Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a", "scope-b"},
		Token:  testToken("access-1"),
	}
	require.NoError(t, store.Save(want))

	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "work", got.Name)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, []string{"scope-a", "scope-b"}, got.Scopes)
	assert.Equal(t, "access-1", got.Token.AccessToken)
	assert.Equal(t, "refresh-1", got.Token.RefreshToken)
	assert.Equal(t, "Bearer", got.Token.TokenType)
	assert.WithinDuration(t, want.Token.Expiry, got.Token.Expiry, 0)
}

func TestStoreGetMissing(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get("ghost")
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.Contains(t, err.Error(), "ghost")
}

func TestStoreInvalidNames(t *testing.T) {
	store := newTestStore(t)

	for _, table := range []struct {
		name string
		call func() error
	}{
		{"get", func() error { _, err := store.Get(""); return err }},
		{"get dotdot", func() error { _, err := store.Get(".."); return err }},
		{"get slash", func() error { _, err := store.Get("a/b"); return err }},
		{"save", func() error {
			return store.Save(&Account{Name: "a/b", Email: "user@example.com"})
		}},
		{"remove", func() error { return store.Remove("") }},
	} {
		t.Run(table.name, func(t *testing.T) {
			assert.Error(t, table.call())
		})
	}
}

func TestStoreListSorted(t *testing.T) {
	store := newTestStore(t)
	for _, name := range []string{"zeta", "alpha", "mid"} {
		require.NoError(t, store.Save(&Account{
			Name:  name,
			Email: name + "@example.com",
			Token: testToken("access-" + name),
		}))
	}

	accounts, err := store.List()
	require.NoError(t, err)
	require.Len(t, accounts, 3)
	assert.Equal(t, []string{"alpha", "mid", "zeta"}, []string{
		accounts[0].Name, accounts[1].Name, accounts[2].Name,
	})
}

func TestStoreListEmptyWithoutAccountsDir(t *testing.T) {
	store := newTestStore(t)

	accounts, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

// TestStoreSaveDedupesByEmail: saving an account whose email already exists
// must update the existing record under its original name, not add a second one.
func TestStoreSaveDedupesByEmail(t *testing.T) {
	store := newTestStore(t)
	first := &Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  testToken("access-1"),
	}
	require.NoError(t, store.Save(first))

	dup := &Account{
		Name:   "alt",
		Email:  "user@example.com",
		Scopes: []string{"scope-b"},
		Token:  testToken("access-2"),
	}
	require.NoError(t, store.Save(dup))
	assert.Equal(t, "work", dup.Name, "deduped save keeps the original name")

	accounts, err := store.List()
	require.NoError(t, err)
	require.Len(t, accounts, 1, "same email must not create a duplicate account")
	assert.Equal(t, "work", accounts[0].Name)
	assert.Equal(t, "access-2", accounts[0].Token.AccessToken)
	assert.Equal(t, []string{"scope-b"}, accounts[0].Scopes)

	_, err = store.Get("alt")
	require.Error(t, err, "no alt.json file may be created")
}

func TestStoreRemove(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("a")}))

	require.NoError(t, store.Remove("work"))

	_, err := store.Get("work")
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	err = store.Remove("work")
	require.Error(t, err)
}

func TestDefaultAccountLifecycle(t *testing.T) {
	store := newTestStore(t)

	def, err := store.DefaultAccount()
	require.NoError(t, err)
	assert.Empty(t, def, "unset default is empty, not an error")

	require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("a")}))
	require.NoError(t, store.SetDefaultAccount("work"))

	def, err = store.DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "work", def)

	err = store.SetDefaultAccount("ghost")
	require.Error(t, err, "default must reference an existing account")
}

func TestStoreRemoveClearsDefault(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{Name: "work", Email: "user@example.com", Token: testToken("a")}))
	require.NoError(t, store.SetDefaultAccount("work"))

	require.NoError(t, store.Remove("work"))

	def, err := store.DefaultAccount()
	require.NoError(t, err)
	assert.Empty(t, def)
}

// TestAccountFileJSONShape pins the on-disk snake_case schema of an account.
func TestAccountFileJSONShape(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(&Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  testToken("access-1"),
	}))

	data, err := afero.ReadFile(store.fs, store.AccountPath("work"))
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.ElementsMatch(t, []string{"name", "provider", "email", "scopes", "token"}, keysOf(raw))
	assert.Equal(t, "google", raw["provider"])
	token, ok := raw["token"].(map[string]any)
	require.True(t, ok, "token must be an object")
	assert.ElementsMatch(t,
		[]string{"access_token", "refresh_token", "token_type", "expiry"},
		keysOf(token))
}
