package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// reauthResult carries Reauth's outcome from its goroutine to the test.
type reauthResult struct {
	acct *config.Account
	err  error
}

// seedReauthAccount saves an account record directly so a Reauth test starts
// from a stored account without running the flow, and returns the store's
// in-memory FS for byte-level assertions that a failed re-auth left the
// account file alone.
func seedReauthAccount(t *testing.T, acct *config.Account) (afero.Fs, *config.Store) {
	t.Helper()
	fs := afero.NewMemMapFs()
	store, err := config.NewStore(fs, "/config")
	require.NoError(t, err)
	require.NoError(t, store.Save(acct))
	return fs, store
}

// staleToken is the pre-reauth stored token: expired so a later refresh
// would touch it, giving byte-level "nothing was saved" assertions teeth.
func staleToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
}

// runReauth drives Reauth hermetically: it starts the call in the
// background, waits for the flow to print its authorization URL, then acts
// as the browser and hits the loopback callback with the stubbed state.
func runReauth(t *testing.T, hooks *flowHooks, reauth func() (*config.Account, error)) (*config.Account, error) {
	t.Helper()
	res := make(chan reauthResult, 1)
	go func() {
		acct, err := reauth()
		res <- reauthResult{acct: acct, err: err}
	}()

	authURL := waitAuthURL(t, hooks.output)
	callback := redirectCallback(t, authURL, "state-123")
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	got := <-res
	return got.acct, got.err
}

// TestOAuthStrategyReauthKeepsNarrowScopes: re-authorizing with empty
// ReauthOptions must reuse the account's granted scopes — a deliberately
// narrowed grant survives re-auth — and update the account file in place
// under the same name with the fresh token.
func TestOAuthStrategyReauthKeepsNarrowScopes(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, store := seedReauthAccount(t, &config.Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{ScopeUserEmail},
		Token:  staleToken(),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)
	acct, err := store.Get("work")
	require.NoError(t, err)

	var mu sync.Mutex
	var gotScopes []string
	hooks.exchangeFn = func(conf *oauth2.Config, _, _ string) (*oauth2.Token, error) {
		mu.Lock()
		defer mu.Unlock()
		gotScopes = conf.Scopes
		return &oauth2.Token{AccessToken: "access-1", Expiry: time.Now().Add(time.Hour)}, nil
	}

	got, err := runReauth(t, hooks, func() (*config.Account, error) {
		return s.Reauth(context.Background(), fs, store, acct, ReauthOptions{})
	})
	require.NoError(t, err)
	assert.Equal(t, "work", got.Name, "the account must be updated under its original name")
	assert.Equal(t, "user@example.com", got.Email)
	require.NotNil(t, got.Token)
	assert.Equal(t, "access-1", got.Token.AccessToken)

	mu.Lock()
	assert.Equal(t, []string{ScopeUserEmail}, gotScopes,
		"the flow must receive exactly the account's granted scopes")
	mu.Unlock()

	persisted, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "access-1", persisted.Token.AccessToken,
		"the fresh token must replace the stale one in the account file")
	assert.Equal(t, []string{ScopeUserEmail}, persisted.Scopes)
	assert.Equal(t, "user@example.com", persisted.Email)
}

// TestOAuthStrategyReauthScopeOverride: explicit ReauthOptions.Scopes reach
// the flow and are what gets persisted, replacing the account's old grant.
func TestOAuthStrategyReauthScopeOverride(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, store := seedReauthAccount(t, &config.Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  staleToken(),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)
	acct, err := store.Get("work")
	require.NoError(t, err)

	var mu sync.Mutex
	var gotScopes []string
	hooks.exchangeFn = func(conf *oauth2.Config, _, _ string) (*oauth2.Token, error) {
		mu.Lock()
		defer mu.Unlock()
		gotScopes = conf.Scopes
		return &oauth2.Token{AccessToken: "access-1", Expiry: time.Now().Add(time.Hour)}, nil
	}

	got, err := runReauth(t, hooks, func() (*config.Account, error) {
		return s.Reauth(context.Background(), fs, store, acct, ReauthOptions{Scopes: []string{"scope-b"}})
	})
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, []string{"scope-b", ScopeUserEmail}, gotScopes,
		"the override must reach the flow (email scope auto-appended for identity)")
	mu.Unlock()
	assert.Equal(t, []string{"scope-b"}, got.Scopes,
		"the persisted grant is the override, not the auto-appended email scope")

	persisted, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, []string{"scope-b"}, persisted.Scopes)
}

// TestOAuthStrategyReauthEmptyAccountScopes: a stored account with no scopes
// at all falls back to the profile's default scope set.
func TestOAuthStrategyReauthEmptyAccountScopes(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, store := seedReauthAccount(t, &config.Account{
		Name:  "work",
		Email: "user@example.com",
		Token: staleToken(),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)
	acct, err := store.Get("work")
	require.NoError(t, err)

	_, err = runReauth(t, hooks, func() (*config.Account, error) {
		return s.Reauth(context.Background(), fs, store, acct, ReauthOptions{})
	})
	require.NoError(t, err)

	persisted, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, GoogleOAuth.DefaultScopes, persisted.Scopes)
}

// TestOAuthStrategyReauthIdentityMismatch: authorizing as a different email
// than the account's pinned identity must fail without saving — the account
// file on disk keeps its old token byte-for-byte — and point at `account
// add` for onboarding the other identity.
func TestOAuthStrategyReauthIdentityMismatch(t *testing.T) {
	hooks := stubFlowSeams(t)
	hooks.emailFn = func(*oauth2.Token) (string, error) { return "other@example.com", nil }
	fs, store := seedReauthAccount(t, &config.Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  staleToken(),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)
	acct, err := store.Get("work")
	require.NoError(t, err)

	before, err := afero.ReadFile(fs, store.AccountPath("work"))
	require.NoError(t, err)

	_, err = runReauth(t, hooks, func() (*config.Account, error) {
		return s.Reauth(context.Background(), fs, store, acct, ReauthOptions{})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "other@example.com")
	assert.Contains(t, err.Error(), "user@example.com")
	assert.Contains(t, err.Error(), "work")
	assert.Contains(t, err.Error(), "account add")

	after, err := afero.ReadFile(fs, store.AccountPath("work"))
	require.NoError(t, err)
	assert.Equal(t, before, after, "a mismatched identity must leave the account file untouched")
	persisted, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "old-access", persisted.Token.AccessToken)
}

// TestOAuthStrategyReauthFlowError: a failing exchange propagates and saves
// nothing.
func TestOAuthStrategyReauthFlowError(t *testing.T) {
	hooks := stubFlowSeams(t)
	hooks.exchangeFn = func(*oauth2.Config, string, string) (*oauth2.Token, error) {
		return nil, errors.New("bad code")
	}
	fs, store := seedReauthAccount(t, &config.Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  staleToken(),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)
	acct, err := store.Get("work")
	require.NoError(t, err)

	before, err := afero.ReadFile(fs, store.AccountPath("work"))
	require.NoError(t, err)

	_, err = runReauth(t, hooks, func() (*config.Account, error) {
		return s.Reauth(context.Background(), fs, store, acct, ReauthOptions{})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exchanging authorization code")

	after, err := afero.ReadFile(fs, store.AccountPath("work"))
	require.NoError(t, err)
	assert.Equal(t, before, after, "a failed flow must leave the account file untouched")
}

// TestOAuthStrategyReauthNoCredentials: a strategy without client
// credentials fails before any flow runs.
func TestOAuthStrategyReauthNoCredentials(t *testing.T) {
	fs, store := seedReauthAccount(t, &config.Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  staleToken(),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, ClientCredentials{})
	acct, err := store.Get("work")
	require.NoError(t, err)

	before, err := afero.ReadFile(fs, store.AccountPath("work"))
	require.NoError(t, err)

	_, err = s.Reauth(context.Background(), fs, store, acct, ReauthOptions{})
	require.EqualError(t, err, "no OAuth client credentials")

	after, err := afero.ReadFile(fs, store.AccountPath("work"))
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
