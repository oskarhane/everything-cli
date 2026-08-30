package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/oskarhane/google-cli/internal/config"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// stubTokenEndpoint swaps the credentials-config seam for one whose token
// endpoint is a local httptest server, and records every token request.
func stubTokenEndpoint(t *testing.T) *tokenEndpointRecorder {
	t.Helper()
	rec := &tokenEndpointRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rec.mu.Lock()
		rec.forms = append(rec.forms, r.PostForm)
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		// Deliberately omits refresh_token: Google often does on refresh.
		_, _ = fmt.Fprintln(w, `{"access_token":"new-access","expires_in":3600,"token_type":"Bearer"}`)
	}))
	t.Cleanup(srv.Close)

	saved := credentialsConfig
	t.Cleanup(func() { credentialsConfig = saved })
	credentialsConfig = func(data []byte, scopes ...string) (*oauth2.Config, error) {
		return &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  srv.URL + "/auth",
				TokenURL: srv.URL + "/token",
			},
			Scopes: scopes,
		}, nil
	}
	return rec
}

type tokenEndpointRecorder struct {
	mu    sync.Mutex
	forms []url.Values
}

func (r *tokenEndpointRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.forms)
}

func (r *tokenEndpointRecorder) form(t *testing.T) url.Values {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Len(t, r.forms, 1)
	return r.forms[0]
}

func saveAccountWithToken(t *testing.T, store *config.Store, token *oauth2.Token) {
	t.Helper()
	require.NoError(t, store.Save(&config.Account{
		Name:   "work",
		Email:  "user@example.com",
		Scopes: []string{"scope-a"},
		Token:  token,
	}))
}

// TestTokenSourceRefreshPersistsToken: an expired stored token must be
// refreshed against the token endpoint and the new token written back to
// the account file, keeping the previous refresh token.
func TestTokenSourceRefreshPersistsToken(t *testing.T) {
	rec := stubTokenEndpoint(t)
	store := newTestStore(t)
	saveAccountWithToken(t, store, &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour), // expired: forces refresh
	})

	fs, credentialsPath := writeCredentialsFile(t)
	ts, err := TokenSource(fs, store, credentialsPath, "work")
	require.NoError(t, err)

	tok, err := ts.Token()
	require.NoError(t, err)
	assert.Equal(t, "new-access", tok.AccessToken)
	assert.Equal(t, "old-refresh", tok.RefreshToken,
		"a refresh response without a new refresh token must keep the old one")

	// The refresh grant hit the mocked endpoint with the stored refresh token.
	form := rec.form(t)
	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "old-refresh", form.Get("refresh_token"))

	// The refreshed token is persisted back to the account file.
	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "new-access", got.Token.AccessToken)
	assert.Equal(t, "old-refresh", got.Token.RefreshToken)

	// A still-valid token is reused without hitting the endpoint again.
	again, err := ts.Token()
	require.NoError(t, err)
	assert.Equal(t, "new-access", again.AccessToken)
	assert.Equal(t, 1, rec.count(), "valid tokens must be reused, not re-fetched")
}

func TestTokenSourceUnknownAccount(t *testing.T) {
	stubTokenEndpoint(t)
	store := newTestStore(t)

	_, err := TokenSource(afero.NewMemMapFs(), store, "/creds.json", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestTokenSourceMissingCredentialsFile(t *testing.T) {
	stubTokenEndpoint(t)
	store := newTestStore(t)
	saveAccountWithToken(t, store, &oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(time.Hour)})

	_, err := TokenSource(afero.NewMemMapFs(), store, "/missing/credentials.json", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading credentials")
}
