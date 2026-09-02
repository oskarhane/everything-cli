package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// The pure registry tests live in internal/redact next to the registry; the
// tests here pin the auth mint/read points that register secrets.

// TestSaveAccountRegistersTokenSecrets: the save point registers both token
// values so a caller persisting a token minted outside RunFlowWith is still
// covered.
func TestSaveAccountRegistersTokenSecrets(t *testing.T) {
	store := newTestStore(t)
	tok := &oauth2.Token{
		AccessToken:  "save-minted-access-xyz",
		RefreshToken: "save-minted-refresh-xyz",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	_, err := SaveAccount(store, "work", "user@example.com", []string{"scope-a"}, tok)
	require.NoError(t, err)
	assert.Equal(t, "***", Redact("save-minted-access-xyz"))
	assert.Equal(t, "***", Redact("save-minted-refresh-xyz"))
}

// TestTokenSourceRegistersStoredTokenSecrets: the read point registers the
// stored token's values when an account is loaded for a token source.
func TestTokenSourceRegistersStoredTokenSecrets(t *testing.T) {
	stubTokenEndpoint(t)
	store := newTestStore(t)
	saveAccountWithToken(t, store, &oauth2.Token{
		AccessToken:  "stored-access-abc",
		RefreshToken: "stored-refresh-abc",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	})
	_, err := TokenSourceWith(store, testClientCredentials, "work", GoogleOAuth)
	require.NoError(t, err)
	assert.Equal(t, "***", Redact("stored-access-abc"))
	assert.Equal(t, "***", Redact("stored-refresh-abc"))
}

// TestRefreshRegistersMintedTokenSecrets: the refresh mint point registers
// the newly minted token's values (stubTokenEndpoint issues "new-access").
func TestRefreshRegistersMintedTokenSecrets(t *testing.T) {
	stubTokenEndpoint(t)
	store := newTestStore(t)
	saveAccountWithToken(t, store, &oauth2.Token{
		AccessToken:  "old-access-def",
		RefreshToken: "old-refresh-def",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	})
	ts, err := TokenSourceWith(store, testClientCredentials, "work", GoogleOAuth)
	require.NoError(t, err)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "new-access", tok.AccessToken)
	assert.Equal(t, "***", Redact("new-access"),
		"the refreshed token must be registered at the mint point")
}
