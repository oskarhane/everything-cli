package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// TestRedactMasksRegisteredSecrets: a registered secret never survives
// Redact; unregistered text passes through untouched.
func TestRedactMasksRegisteredSecrets(t *testing.T) {
	RegisterSecret("redact-test-access-token")
	assert.Equal(t, "header *** trailer", Redact("header redact-test-access-token trailer"))
	assert.Equal(t, "nothing secret here", Redact("nothing secret here"))
	assert.Equal(t, "", Redact(""))
}

// TestRegisterSecretIgnoresEmpty: an absent token value must not redact
// every string.
func TestRegisterSecretIgnoresEmpty(t *testing.T) {
	RegisterSecret("")
	assert.Equal(t, "still here", Redact("still here"))
}

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
	fs, credentialsPath := writeCredentialsFile(t)

	_, err := TokenSourceWith(fs, store, credentialsPath, "work", GoogleOAuth)
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
	fs, credentialsPath := writeCredentialsFile(t)

	ts, err := TokenSourceWith(fs, store, credentialsPath, "work", GoogleOAuth)
	require.NoError(t, err)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "new-access", tok.AccessToken)
	assert.Equal(t, "***", Redact("new-access"),
		"the refreshed token must be registered at the mint point")
}
