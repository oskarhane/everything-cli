package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// serveOnce starts a test server that records the Authorization header of
// the request it receives into *authHeader.
func serveOnce(t *testing.T, authHeader *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newCompositeTestStrategy builds the composite strategy on an in-memory
// FS and store.
func newCompositeTestStrategy(t *testing.T) (*strategy, afero.Fs, *config.Store) {
	t.Helper()
	fs := afero.NewMemMapFs()
	store, err := config.NewStore(fs, "/config")
	require.NoError(t, err)
	return newStrategy(fs, store), fs, store
}

// TestCompositeAddDefaultsToAPIKey: without UseOAuth, Add captures the API
// key exactly as before the OAuth option existed.
func TestCompositeAddDefaultsToAPIKey(t *testing.T) {
	s, fs, store := newCompositeTestStrategy(t)

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{
		Name:   "work",
		APIKey: "test-key-123",
	})
	require.NoError(t, err)
	assert.Equal(t, ID, acct.Provider)
	assert.Nil(t, acct.Token, "the API-key path must not mint a token")

	var payload struct {
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal(acct.Auth, &payload))
	assert.Equal(t, "test-key-123", payload.APIKey)
}

// TestCompositeAddDispatchesOAuth: UseOAuth routes Add to the OAuth path.
func TestCompositeAddDispatchesOAuth(t *testing.T) {
	fake := newFakeLinear(t)
	s, fs, store := newCompositeTestStrategy(t)
	s.oauth.profile = fake.profileOver(s.oauth.profile)
	s.oauth.graphqlURL = fake.graphqlURL
	s.oauth.getenv = func(string) string { return "" }
	s.oauth.runFlow = func(afero.Fs, string, []string, auth.OAuthProfile) (*oauth2.Token, string, error) {
		return &oauth2.Token{AccessToken: "fake-access", Expiry: time.Now().Add(time.Hour)},
			"viewer@example.com", nil
	}

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{
		Name:     "work",
		UseOAuth: true,
		ClientID: "test-client-id",
	})
	require.NoError(t, err)
	assert.Equal(t, ID, acct.Provider)
	require.NotNil(t, acct.Token, "the OAuth path must persist a token")
}

// TestCompositeClientDispatchesOnAccountShape: API-key accounts get the
// static header client, OAuth accounts the refreshing Bearer client.
func TestCompositeClientDispatchesOnAccountShape(t *testing.T) {
	fake := newFakeLinear(t)
	s, _, store := newCompositeTestStrategy(t)
	s.oauth.profile = fake.profileOver(s.oauth.profile)

	keyPayload, err := json.Marshal(map[string]string{"api_key": "test-key-123"})
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{
		Name: "bykey", Provider: ID, Auth: keyPayload,
	}))
	oauthPayload, err := json.Marshal(oauthAuthPayload{ClientID: "test-client-id"})
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{
		Name: "byoauth", Provider: ID, Email: "viewer@example.com",
		Token: &oauth2.Token{
			AccessToken: "still-valid", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour),
		},
		Auth: oauthPayload,
	}))

	keyAcct, err := store.GetProvider(ID, "bykey")
	require.NoError(t, err)
	keyClient, err := s.Client(context.Background(), keyAcct)
	require.NoError(t, err)

	oauthAcct, err := store.GetProvider(ID, "byoauth")
	require.NoError(t, err)
	oauthClient, err := s.Client(context.Background(), oauthAcct)
	require.NoError(t, err)

	var keyAuth, oauthAuthHeader string
	srv := serveOnce(t, &keyAuth)
	resp, err := keyClient.Get(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, "test-key-123", keyAuth, "API-key accounts send the raw key, no Bearer prefix")

	srv2 := serveOnce(t, &oauthAuthHeader)
	resp, err = oauthClient.Get(srv2.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, "Bearer still-valid", oauthAuthHeader, "OAuth accounts send the token as Bearer")
}
