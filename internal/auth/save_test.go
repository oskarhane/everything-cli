package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func flowToken(access string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  access,
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
}

// TestSaveAccountDedupesByEmail: adding an account whose email already
// exists updates the existing account (under its original name) instead of
// creating a duplicate, and returns the canonical name.
func TestSaveAccountDedupesByEmail(t *testing.T) {
	store := newTestStore(t)

	name, err := SaveAccount(store, "work", "user@example.com", []string{"scope-a"}, flowToken("access-1"))
	require.NoError(t, err)
	assert.Equal(t, "work", name)

	name, err = SaveAccount(store, "alt", "user@example.com", []string{"scope-b"}, flowToken("access-2"))
	require.NoError(t, err)
	assert.Equal(t, "work", name, "a known email must update the original account")

	accounts, err := store.List()
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "work", accounts[0].Name)
	assert.Equal(t, "access-2", accounts[0].Token.AccessToken)
	assert.Equal(t, []string{"scope-b"}, accounts[0].Scopes)
}

func TestSaveAccountPersistsRecord(t *testing.T) {
	store := newTestStore(t)
	tok := flowToken("access-1")

	name, err := SaveAccount(store, "personal", "me@example.com", []string{"scope-a"}, tok)
	require.NoError(t, err)
	assert.Equal(t, "personal", name)

	got, err := store.Get("personal")
	require.NoError(t, err)
	assert.Equal(t, "me@example.com", got.Email)
	assert.Equal(t, []string{"scope-a"}, got.Scopes)
	assert.Equal(t, "access-1", got.Token.AccessToken)
	assert.Equal(t, "refresh-1", got.Token.RefreshToken)
	assert.WithinDuration(t, tok.Expiry, got.Token.Expiry, 0)
}
