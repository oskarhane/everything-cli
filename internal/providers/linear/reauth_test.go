package linear

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// seedOAuthAccount persists an OAuth-shaped linear account (viewer email,
// narrowed scopes, stale token, client credentials payload) directly in the
// store, returning the loaded record — the state `linear account auth`
// re-authorizes.
func seedOAuthAccount(t *testing.T, store *config.Store, name string) *config.Account {
	t.Helper()
	payload, err := json.Marshal(oauthAuthPayload{ClientID: "test-client-id", ClientSecret: "test-client-secret"})
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{
		Name:     name,
		Provider: ID,
		Email:    "viewer@example.com",
		Scopes:   []string{"read"},
		Token: &oauth2.Token{
			AccessToken:  "stale-access",
			RefreshToken: "stale-refresh",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(-time.Hour),
		},
		Auth: payload,
	}))
	acct, err := store.GetProvider(ID, name)
	require.NoError(t, err)
	return acct
}

// TestOAuthReauthReplacesTokenAndPreservesPayload: re-auth re-runs the flow
// with the account's stored client credentials, resolves the viewer for
// identity, replaces the token under the same name, and preserves the Auth
// payload — the client credentials Client still needs for refresh.
func TestOAuthReauthReplacesTokenAndPreservesPayload(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)
	acct := seedOAuthAccount(t, store, "work")

	updated, err := s.Reauth(context.Background(), fs, store, acct, auth.ReauthOptions{})
	require.NoError(t, err)
	assert.Equal(t, "work", updated.Name)
	assert.Equal(t, "viewer@example.com", updated.Email)
	assert.Equal(t, []string{"read"}, updated.Scopes,
		"the account's narrowed grant survives re-auth")
	require.NotNil(t, updated.Token)
	assert.Equal(t, "fake-access-1", updated.Token.AccessToken)
	assert.Equal(t, "fake-refresh-2", updated.Token.RefreshToken)
	assert.Equal(t, map[string]string{
		"id": "user-1", "name": "Test User", "email": "viewer@example.com",
	}, updated.Identity, "the viewer is captured for the Identity field")

	// The stored Auth payload survives untouched.
	var payload oauthAuthPayload
	require.NoError(t, json.Unmarshal(updated.Auth, &payload))
	assert.Equal(t, "test-client-id", payload.ClientID)
	assert.Equal(t, "test-client-secret", payload.ClientSecret)

	// Persisted under the same name, through the real store.
	persisted, err := store.GetProvider(ID, "work")
	require.NoError(t, err)
	assert.Equal(t, "fake-access-1", persisted.Token.AccessToken)
	require.NoError(t, json.Unmarshal(persisted.Auth, &payload))
	assert.Equal(t, "test-client-secret", payload.ClientSecret)

	// The token exchange carried the account's stored client credentials.
	fake.mu.Lock()
	require.Len(t, fake.tokenForms, 1)
	form := fake.tokenForms[0]
	tokenAuth := fake.tokenAuths[0]
	fake.mu.Unlock()
	gotID, gotSecret := clientCredentials(t, form, tokenAuth)
	assert.Equal(t, "test-client-id", gotID)
	assert.Equal(t, "test-client-secret", gotSecret)
}

// TestOAuthReauthScopeOverride: explicit ReauthOptions.Scopes reach the
// flow and are persisted, overriding the account's granted scopes.
func TestOAuthReauthScopeOverride(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)
	acct := seedOAuthAccount(t, store, "work")

	updated, err := s.Reauth(context.Background(), fs, store, acct, auth.ReauthOptions{Scopes: []string{"read", "write"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, updated.Scopes)

	persisted, err := store.GetProvider(ID, "work")
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, persisted.Scopes)
}

// TestOAuthReauthIdentityMismatchSavesNothing: identity is pinned — the
// browser authorizing a different email fails with guidance toward
// `linear account add`, and nothing is written.
func TestOAuthReauthIdentityMismatchSavesNothing(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)
	acct := seedOAuthAccount(t, store, "work")
	// Pin the account to a different identity than the one the (fake)
	// browser session will authorize.
	acct.Email = "other@example.com"

	_, err := s.Reauth(context.Background(), fs, store, acct, auth.ReauthOptions{})
	require.ErrorContains(t, err, `authorized identity "viewer@example.com" does not match account "work" ("other@example.com")`)
	require.ErrorContains(t, err, "linear account add")

	// Nothing was saved: the stored account keeps its stale token.
	persisted, err := store.GetProvider(ID, "work")
	require.NoError(t, err)
	require.NotNil(t, persisted.Token)
	assert.Equal(t, "stale-access", persisted.Token.AccessToken)
	assert.Equal(t, "viewer@example.com", persisted.Email)
}

// TestOAuthReauthRequiresStoredClientID: an account whose payload carries
// no OAuth client ID has nothing to run a flow with.
func TestOAuthReauthRequiresStoredClientID(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)
	require.NoError(t, store.Save(&config.Account{
		Name:     "work",
		Provider: ID,
		Email:    "viewer@example.com",
		Token:    &oauth2.Token{AccessToken: "stale-access"},
		Auth:     json.RawMessage(`{}`),
	}))
	acct, err := store.GetProvider(ID, "work")
	require.NoError(t, err)

	_, err = s.Reauth(context.Background(), fs, store, acct, auth.ReauthOptions{})
	require.ErrorContains(t, err, `account "work" holds no OAuth client ID`)
	assert.Equal(t, 0, fake.graphqlHits, "no flow may run without a client ID")
}

// TestCompositeReauthDispatchesOnAccountShape: like Client, Reauth routes
// OAuth accounts (those carrying a token) to the OAuth path and rejects
// API-key accounts with re-creation guidance instead of a flow.
func TestCompositeReauthDispatchesOnAccountShape(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newCompositeTestStrategy(t)
	s.oauth.profile = fake.profileOver(s.oauth.profile)
	s.oauth.graphqlURL = fake.graphqlURL
	s.oauth.getenv = func(string) string { return "" }
	flowRan := false
	s.oauth.runFlow = func(auth.ClientCredentials, []string, auth.OAuthProfile) (*oauth2.Token, string, error) {
		flowRan = true
		return &oauth2.Token{AccessToken: "fresh-access", Expiry: time.Now().Add(time.Hour)},
			"viewer@example.com", nil
	}

	oauthAcct := seedOAuthAccount(t, store, "byoauth")
	updated, err := s.Reauth(context.Background(), fs, store, oauthAcct, auth.ReauthOptions{})
	require.NoError(t, err)
	require.NotNil(t, updated.Token)
	assert.Equal(t, "fresh-access", updated.Token.AccessToken)
	assert.True(t, flowRan)

	keyPayload, err := json.Marshal(map[string]string{"api_key": "test-key-123"})
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{
		Name:     "bykey",
		Provider: ID,
		Auth:     keyPayload,
	}))
	keyAcct, err := store.GetProvider(ID, "bykey")
	require.NoError(t, err)

	_, err = s.Reauth(context.Background(), fs, store, keyAcct, auth.ReauthOptions{})
	require.ErrorContains(t, err, `account "bykey" uses an API key, not OAuth`)
	require.ErrorContains(t, err, "linear account add")
}

// TestAccountAuthLeafRejectsAPIKeyAccount runs the real command tree with
// the production strategy factory: an API-key account reaches the leaf,
// and the composite's re-creation guidance surfaces as the command's error.
func TestAccountAuthLeafRejectsAPIKeyAccount(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "bykey") // API-key payload, no token

	root := newLinearCmd(cfg)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"account", "auth", "bykey"})
	err := root.Execute()
	require.ErrorContains(t, err, `account "bykey" uses an API key, not OAuth`)
	require.ErrorContains(t, err, "linear account add")
}
