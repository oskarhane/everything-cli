package linear

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// fakeLinear is a hermetic Linear: a fake OAuth token endpoint and a fake
// GraphQL endpoint answering the viewer query.
type fakeLinear struct {
	mu           sync.Mutex
	tokenForms   []url.Values
	tokenAuths   []string
	viewerAuths  []string
	graphqlHits  int
	tokenSrv     *httptest.Server
	graphqlSrv   *httptest.Server
	tokenURL     string
	graphqlURL   string
	accessToken  string
	refreshToken string
}

// newFakeLinear starts the fake endpoints. The token endpoint records the
// exchange/refresh form and answers with fresh fake tokens; the GraphQL
// endpoint records the Authorization header and answers the viewer query.
func newFakeLinear(t *testing.T) *fakeLinear {
	t.Helper()
	f := &fakeLinear{accessToken: "fake-access-1", refreshToken: "fake-refresh-1"}
	f.tokenSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokenForms = append(f.tokenForms, r.PostForm)
		f.tokenAuths = append(f.tokenAuths, r.Header.Get("Authorization"))
		if r.PostForm.Get("grant_type") == "refresh_token" {
			f.accessToken = "fake-access-2"
		}
		access := f.accessToken
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"refresh_token":"fake-refresh-2","token_type":"Bearer","expires_in":86399,"scope":"read write"}`, access)
	}))
	f.graphqlSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.viewerAuths = append(f.viewerAuths, r.Header.Get("Authorization"))
		f.graphqlHits++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"viewer":{"id":"user-1","name":"Test User","email":"viewer@example.com"}}}`)
	}))
	t.Cleanup(f.tokenSrv.Close)
	t.Cleanup(f.graphqlSrv.Close)
	f.tokenURL = f.tokenSrv.URL + "/oauth/token"
	f.graphqlURL = f.graphqlSrv.URL + "/graphql"
	return f
}

// clientCredentials extracts the client ID and secret from a token
// request: Linear accepts them as form params or as HTTP Basic auth, and
// the oauth2 client picks per endpoint.
func clientCredentials(t *testing.T, form url.Values, authHeader string) (id, secret string) {
	t.Helper()
	id, secret = form.Get("client_id"), form.Get("client_secret")
	if id == "" && strings.HasPrefix(authHeader, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		require.NoError(t, err)
		parts := strings.SplitN(string(decoded), ":", 2)
		id = parts[0]
		if len(parts) > 1 {
			secret = parts[1]
		}
	}
	return id, secret
}

// profileOver returns the pinned Linear profile with its endpoints
// retargeted at the fake servers — the tests' stand-in for the pinned
// production endpoints.
func (f *fakeLinear) profileOver(base auth.OAuthProfile) auth.OAuthProfile {
	base.Endpoint = oauth2.Endpoint{
		AuthURL:  f.tokenSrv.URL + "/oauth/authorize",
		TokenURL: f.tokenURL,
	}
	base.UserinfoURL = f.graphqlURL
	return base
}

// newOAuthTestStrategy builds the OAuth strategy on an in-memory FS and
// store, its endpoints pointed at the fake Linear, its flow seam running a
// real HTTP code exchange against the fake token endpoint and the real
// identity resolution against the fake GraphQL endpoint.
func newOAuthTestStrategy(t *testing.T, f *fakeLinear) (*OAuthStrategy, afero.Fs, *config.Store) {
	t.Helper()
	fs := afero.NewMemMapFs()
	store, err := config.NewStore(fs, "/config")
	require.NoError(t, err)
	s := newOAuthStrategy(fs, store)
	s.profile = f.profileOver(s.profile)
	s.graphqlURL = f.graphqlURL
	s.getenv = func(string) string { return "" }
	s.runFlow = func(flowFs afero.Fs, credentialsPath string, scopes []string, profile auth.OAuthProfile) (*oauth2.Token, string, error) {
		// A hermetic stand-in for the browser flow: the browser leg is
		// covered by internal/auth flow tests. The exchange runs for real
		// against the profile's pinned endpoint — if endpoints came from
		// anywhere else (a credentials file), this would miss the fake.
		data, err := afero.ReadFile(flowFs, credentialsPath)
		require.NoError(t, err)
		var creds struct {
			Installed struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
			} `json:"installed"`
		}
		require.NoError(t, json.Unmarshal(data, &creds))
		require.Equal(t, "test-client-id", creds.Installed.ClientID,
			"the flow must receive the captured client ID")
		conf := &oauth2.Config{
			ClientID:     creds.Installed.ClientID,
			ClientSecret: creds.Installed.ClientSecret,
			Endpoint:     profile.Endpoint,
			Scopes:       scopes,
			RedirectURL:  "http://localhost:1",
		}
		tok, err := conf.Exchange(context.Background(), "fake-code",
			oauth2.SetAuthURLParam("code_verifier", "fake-verifier"))
		require.NoError(t, err)
		email, err := profile.IdentityResolver(context.Background(), tok)
		require.NoError(t, err)
		return tok, email, nil
	}
	return s, fs, store
}

// TestLinearOAuthProfilePinsEndpoints: the OAuth path's endpoints, scopes
// and identity endpoint are pinned in the provider's profile — a
// user-supplied file has no say in where tokens go.
func TestLinearOAuthProfilePinsEndpoints(t *testing.T) {
	assert.Equal(t, "https://linear.app/oauth/authorize", linearOAuthProfile.Endpoint.AuthURL)
	assert.Equal(t, "https://api.linear.app/oauth/token", linearOAuthProfile.Endpoint.TokenURL)
	assert.Equal(t, "https://api.linear.app/graphql", linearOAuthProfile.UserinfoURL)
	assert.Equal(t, []string{"read", "write"}, linearOAuthProfile.DefaultScopes)
	assert.Equal(t, "read", linearOAuthProfile.EmailScope)
	assert.Equal(t, ",", linearOAuthProfile.ScopeSeparator,
		"Linear's authorize endpoint documents a comma-separated scope list")

	// The generated client-credentials document carries ONLY the client
	// credentials — no auth_uri/token_uri a tampered file could exploit.
	fs := afero.NewMemMapFs()
	path, cleanup, err := writeClientCredentials(fs, "id-1", "secret-1")
	require.NoError(t, err)
	defer cleanup()
	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "auth_uri")
	assert.NotContains(t, string(data), "token_uri")
}

// TestOAuthAddRunsFlowAndPersistsLinearAccount: the OAuth path exchanges
// the code against the (fake) pinned token endpoint, resolves identity via
// the viewer query, and persists a linear-provider account with token,
// identity and client credentials.
func TestOAuthAddRunsFlowAndPersistsLinearAccount(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{
		Name:         "work",
		UseOAuth:     true,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	})
	require.NoError(t, err)
	assert.Equal(t, "work", acct.Name)
	assert.Equal(t, ID, acct.Provider)
	assert.Equal(t, "viewer@example.com", acct.Email)
	assert.Equal(t, []string{"read", "write"}, acct.Scopes)
	require.NotNil(t, acct.Token)
	assert.Equal(t, "fake-access-1", acct.Token.AccessToken)
	assert.Equal(t, map[string]string{
		"id": "user-1", "name": "Test User", "email": "viewer@example.com",
	}, acct.Identity)

	// The token exchange hit the profile's pinned (fake) token endpoint
	// with the PKCE verifier and the client credentials.
	fake.mu.Lock()
	require.Len(t, fake.tokenForms, 1)
	form := fake.tokenForms[0]
	tokenAuth := fake.tokenAuths[0]
	fake.mu.Unlock()
	assert.Equal(t, "authorization_code", form.Get("grant_type"))
	assert.Equal(t, "fake-code", form.Get("code"))
	assert.Equal(t, "fake-verifier", form.Get("code_verifier"), "the flow is PKCE")
	gotID, gotSecret := clientCredentials(t, form, tokenAuth)
	assert.Equal(t, "test-client-id", gotID)
	assert.Equal(t, "test-client-secret", gotSecret)

	// Identity came from the viewer query, authorized with the Bearer token.
	fake.mu.Lock()
	require.Len(t, fake.viewerAuths, 1)
	assert.Equal(t, "Bearer fake-access-1", fake.viewerAuths[0])
	fake.mu.Unlock()

	// Persisted under the provider-scoped store, as the provider default.
	persisted, err := store.GetProvider(ID, "work")
	require.NoError(t, err)
	assert.Equal(t, "fake-access-1", persisted.Token.AccessToken)
	var payload oauthAuthPayload
	require.NoError(t, json.Unmarshal(persisted.Auth, &payload))
	assert.Equal(t, "test-client-id", payload.ClientID)
	def, err := store.DefaultAccountFor(ID)
	require.NoError(t, err)
	assert.Equal(t, "work", def)
}

// TestOAuthAddClientIDFromEnv: LINEAR_CLIENT_ID supplies the app
// credentials when --client-id is not passed.
func TestOAuthAddClientIDFromEnv(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)
	s.getenv = func(key string) string {
		if key == envVarClientID {
			return "env-client-id"
		}
		return ""
	}
	// The env client ID must reach the flow's credentials document.
	s.runFlow = func(flowFs afero.Fs, credentialsPath string, scopes []string, profile auth.OAuthProfile) (*oauth2.Token, string, error) {
		data, err := afero.ReadFile(flowFs, credentialsPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "env-client-id")
		tok := &oauth2.Token{AccessToken: "fake-access", Expiry: time.Now().Add(time.Hour)}
		email, err := profile.IdentityResolver(context.Background(), tok)
		require.NoError(t, err)
		return tok, email, nil
	}

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "work", UseOAuth: true})
	require.NoError(t, err)
	assert.Equal(t, "viewer@example.com", acct.Email)
}

// TestOAuthAddRequiresClientID: without flag or env, the OAuth path fails
// before any flow starts.
func TestOAuthAddRequiresClientID(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newOAuthTestStrategy(t, fake)

	_, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "work", UseOAuth: true})
	require.ErrorContains(t, err, "--client-id")
	require.ErrorContains(t, err, envVarClientID)
	assert.Equal(t, 0, fake.graphqlHits, "no flow may run without a client ID")
}

// TestOAuthClientRefreshesAgainstPinnedEndpoint: an expired OAuth token
// refreshes against the profile's pinned token endpoint and the refreshed
// token persists back to the linear account file; API calls bear the
// OAuth token as a Bearer credential.
func TestOAuthClientRefreshesAgainstPinnedEndpoint(t *testing.T) {
	fake := newFakeLinear(t)
	s, _, store := newOAuthTestStrategy(t, fake)
	payload, err := json.Marshal(oauthAuthPayload{ClientID: "test-client-id"})
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{
		Name:     "work",
		Provider: ID,
		Email:    "viewer@example.com",
		Scopes:   []string{"read", "write"},
		Token: &oauth2.Token{
			AccessToken:  "expired-access",
			RefreshToken: "fake-refresh-1",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(-time.Hour),
		},
		Auth: payload,
	}))

	acct, err := store.GetProvider(ID, "work")
	require.NoError(t, err)
	client, err := s.Client(context.Background(), acct)
	require.NoError(t, err)

	// Drive one API request to force the refresh; the resource server
	// records the Authorization header.
	var gotAuth string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(apiSrv.Close)
	resp, err := client.Get(apiSrv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	fake.mu.Lock()
	require.Len(t, fake.tokenForms, 1, "the expired token must be refreshed once")
	form := fake.tokenForms[0]
	tokenAuth := fake.tokenAuths[0]
	fake.mu.Unlock()
	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "fake-refresh-1", form.Get("refresh_token"))
	gotClientID, _ := clientCredentials(t, form, tokenAuth)
	assert.Equal(t, "test-client-id", gotClientID)
	assert.Equal(t, "Bearer fake-access-2", gotAuth, "OAuth tokens use the Bearer scheme")

	persisted, err := store.GetProvider(ID, "work")
	require.NoError(t, err)
	assert.Equal(t, "fake-access-2", persisted.Token.AccessToken,
		"the refreshed token must persist back to the linear account file")
}

// TestQueryViewerSendsBearerAndReadsIdentity: the identity query POSTs the
// viewer selection with the token as a Bearer credential.
func TestQueryViewerSendsBearerAndReadsIdentity(t *testing.T) {
	fake := newFakeLinear(t)

	who, err := queryViewer(context.Background(), fake.graphqlURL, "fake-access-1")
	require.NoError(t, err)
	assert.Equal(t, viewer{ID: "user-1", Name: "Test User", Email: "viewer@example.com"}, who)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.viewerAuths, 1)
	assert.Equal(t, "Bearer fake-access-1", fake.viewerAuths[0])
}

func TestQueryViewerRequiresEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":{"viewer":{"id":"user-1"}}}`)
	}))
	t.Cleanup(srv.Close)

	_, err := queryViewer(context.Background(), srv.URL, "fake-access-1")
	require.ErrorContains(t, err, "no email")
}
