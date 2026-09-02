package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// TestGoogleOAuthProfile: Google's profile pins Google's endpoints and the
// userinfo v2 identity endpoint, and carries the CLI's full default scopes.
func TestGoogleOAuthProfile(t *testing.T) {
	assert.Equal(t, google.Endpoint, GoogleOAuth.Endpoint)
	assert.Equal(t, "https://www.googleapis.com/oauth2/v2/userinfo", GoogleOAuth.UserinfoURL)
	assert.Equal(t, ScopeUserEmail, GoogleOAuth.EmailScope)
	for _, want := range []string{ScopesGmail[0], ScopesCalendar[0], ScopesDrive[0],
		ScopesDocs[0], ScopesSheets[0], ScopesSlides[0], ScopeUserEmail} {
		assert.Contains(t, GoogleOAuth.DefaultScopes, want)
	}
}

// TestParseClientCredentialsIgnoresEndpoints: the parsed shape carries only
// client_id/client_secret — auth_uri/token_uri claimed by a credentials
// file never even enter it, so a profile's pinned endpoints always win, for
// any provider.
func TestParseClientCredentialsIgnoresEndpoints(t *testing.T) {
	data := []byte(`{
	  "installed": {
	    "client_id": "test-client-id",
	    "client_secret": "test-client-secret",
	    "auth_uri": "https://evil.example/auth",
	    "token_uri": "https://evil.example/token",
	    "redirect_uris": ["http://localhost"]
	  }
	}`)

	creds, err := ParseClientCredentials(data)
	require.NoError(t, err)
	assert.Equal(t, ClientCredentials{ID: "test-client-id", Secret: "test-client-secret"}, creds,
		"the shape carries no endpoints a planted file could exploit")
}

// TestOAuthStrategyClientReturnsHTTPClient: Client must hand back an
// *http.Client (the seam), whose transport reuses the account's valid
// stored token without hitting the token endpoint.
func TestOAuthStrategyClientReturnsHTTPClient(t *testing.T) {
	rec := stubTokenEndpoint(t)
	store := newTestStore(t)
	saveAccountWithToken(t, store, &oauth2.Token{
		AccessToken: "still-valid",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)

	acct, err := store.Get("work")
	require.NoError(t, err)
	client, err := s.Client(context.Background(), acct)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, 0, rec.count(), "a valid stored token must be reused, not refreshed")
}

func TestOAuthStrategyClientNilAccount(t *testing.T) {
	s := NewOAuthStrategy(GoogleOAuth, nil, ClientCredentials{})

	_, err := s.Client(context.Background(), nil)
	require.Error(t, err)
}

// TestOAuthStrategyClientRefreshesExpiredToken: an expired stored token is
// refreshed through the strategy and persisted back to the account file.
func TestOAuthStrategyClientRefreshesExpiredToken(t *testing.T) {
	rec := stubTokenEndpoint(t)
	store := newTestStore(t)
	saveAccountWithToken(t, store, &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	})
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)

	acct, err := store.Get("work")
	require.NoError(t, err)
	client, err := s.Client(context.Background(), acct)
	require.NoError(t, err)

	// Drive one request through the client's transport to force the refresh.
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(apiSrv.Close)
	resp, err := client.Get(apiSrv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 1, rec.count(), "the expired token must be refreshed once")
	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "new-access", got.Token.AccessToken,
		"the refreshed token must be persisted back to the account file")
}

// TestOAuthStrategyAdd: Add runs the profile's flow and persists the
// account, exactly like `account add` does today for Google.
func TestOAuthStrategyAdd(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs := afero.NewMemMapFs()
	store := newTestStore(t)
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)

	res := make(chan struct {
		acct *config.Account
		err  error
	}, 1)
	go func() {
		acct, err := s.Add(context.Background(), fs, store, AddOptions{
			Name:        "work",
			Credentials: testClientCredentials,
			Scopes:      []string{"scope-a"},
		})
		res <- struct {
			acct *config.Account
			err  error
		}{acct, err}
	}()

	authURL := waitAuthURL(t, hooks.output)
	callback := redirectCallback(t, authURL, "state-123")
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	got := <-res
	require.NoError(t, got.err)
	assert.Equal(t, "work", got.acct.Name)
	assert.Equal(t, "user@example.com", got.acct.Email)
	require.NotNil(t, got.acct.Token)
	assert.Equal(t, "access-1", got.acct.Token.AccessToken)

	persisted, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "access-1", persisted.Token.AccessToken)
}

// TestOAuthStrategyAddDefaultScopes: empty AddOptions.Scopes falls back to
// the profile's default scope set.
func TestOAuthStrategyAddDefaultScopes(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs := afero.NewMemMapFs()
	store := newTestStore(t)
	s := NewOAuthStrategy(GoogleOAuth, store, testClientCredentials)

	res := make(chan error, 1)
	go func() {
		_, err := s.Add(context.Background(), fs, store, AddOptions{
			Name: "work",
		})
		res <- err
	}()

	authURL := waitAuthURL(t, hooks.output)
	callback := redirectCallback(t, authURL, "state-123")
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.NoError(t, <-res)
	persisted, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, GoogleOAuth.DefaultScopes, persisted.Scopes)
}

// TestRunFlowWithCustomProfile: the generalized flow takes its endpoints,
// identity URL and email scope from the supplied profile — the client
// credentials it is handed carry no endpoints at all.
func TestRunFlowWithCustomProfile(t *testing.T) {
	hooks := stubFlowSeams(t)

	profile := OAuthProfile{
		Name:        "other-cli",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://provider.example/auth", TokenURL: "https://provider.example/token"},
		UserinfoURL: "https://provider.example/userinfo",
		EmailScope:  "provider-email-scope",
	}

	var mu sync.Mutex
	var conf *oauth2.Config
	var gotUserinfoURL string
	hooks.exchangeFn = func(c *oauth2.Config, _, _ string) (*oauth2.Token, error) {
		mu.Lock()
		defer mu.Unlock()
		conf = c
		return &oauth2.Token{AccessToken: "access-1", Expiry: time.Now().Add(time.Hour)}, nil
	}
	fetchEmail = func(_ context.Context, url string, _ *oauth2.Token) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		gotUserinfoURL = url
		return "user@provider.example", nil
	}

	res := make(chan flowResult, 1)
	go func() {
		tok, email, err := RunFlowWith(testClientCredentials, []string{"scope-a"}, profile)
		res <- flowResult{token: tok, email: email, err: err}
	}()

	authURL := waitAuthURL(t, hooks.output)
	assert.Contains(t, authURL, "https://provider.example/auth",
		"the authorization URL must come from the profile, not the credentials file")
	assert.Contains(t, hooks.output.String(), "Authorize other-cli")
	callback := redirectCallback(t, authURL, "state-123")
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	got := <-res
	require.NoError(t, got.err)
	assert.Equal(t, "user@provider.example", got.email)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, conf)
	assert.Equal(t, profile.Endpoint, conf.Endpoint,
		"the token endpoint must come from the profile, not the credentials file")
	assert.Contains(t, conf.Scopes, "scope-a")
	assert.Contains(t, conf.Scopes, "provider-email-scope",
		"the profile's email scope is auto-appended for identity resolution")
	assert.Equal(t, profile.UserinfoURL, gotUserinfoURL)
}
