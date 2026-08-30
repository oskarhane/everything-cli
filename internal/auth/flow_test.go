package auth

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// flowResult carries RunFlow's outcome from its goroutine to the test.
type flowResult struct {
	token *oauth2.Token
	email string
	err   error
}

// startFlow runs RunFlow in the background and returns its result channel.
func startFlow(fs afero.Fs, credentialsPath string, scopes []string) chan flowResult {
	res := make(chan flowResult, 1)
	go func() {
		token, email, err := RunFlow(fs, credentialsPath, scopes)
		res <- flowResult{token: token, email: email, err: err}
	}()
	return res
}

func TestRunFlow(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, credentialsPath := writeCredentialsFile(t)

	var mu sync.Mutex
	var gotCode, gotRedirect string
	var gotScopes []string
	var gotToken *oauth2.Token
	hooks.exchangeFn = func(conf *oauth2.Config, code string) (*oauth2.Token, error) {
		mu.Lock()
		defer mu.Unlock()
		gotCode, gotRedirect, gotScopes = code, conf.RedirectURL, conf.Scopes
		return &oauth2.Token{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}, nil
	}
	hooks.emailFn = func(tok *oauth2.Token) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		gotToken = tok
		return "user@example.com", nil
	}

	res := startFlow(fs, credentialsPath, []string{"scope-a"})

	// Act as the browser: take the printed URL, then hit the redirect URI.
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "state-123", u.Query().Get("state"), "flow must pin a CSRF state")
	assert.Contains(t, u.Query().Get("scope"), "userinfo.email",
		"userinfo.email is auto-appended so the email can be resolved")
	redirect := u.Query().Get("redirect_uri")
	require.NotEmpty(t, redirect, "auth URL must carry the loopback redirect_uri")
	assert.True(t, strings.HasPrefix(redirect, "http://localhost:"),
		"redirect URI must be a localhost loopback, got %q", redirect)

	callback := redirect + "?" + url.Values{"code": {"test-code"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "Authorization complete")

	got := <-res
	require.NoError(t, got.err)
	assert.Equal(t, "access-1", got.token.AccessToken)
	assert.Equal(t, "user@example.com", got.email)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "test-code", gotCode, "the callback code must be exchanged")
	assert.Equal(t, redirect, gotRedirect, "exchange must use the loopback redirect URI")
	assert.Contains(t, gotScopes, "scope-a")
	assert.NotNil(t, gotToken, "userinfo must be called with the exchanged token")
}

func TestRunFlowStateMismatch(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, credentialsPath := writeCredentialsFile(t)

	res := startFlow(fs, credentialsPath, nil)

	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"forged"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	got := <-res
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "state")
	assert.Nil(t, got.token)
}

func TestRunFlowMissingCredentialsFile(t *testing.T) {
	stubFlowSeams(t)

	got := <-startFlow(afero.NewMemMapFs(), "/definitely/not/here/credentials.json", nil)

	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "reading credentials")
}

// TestRunFlowPinsEndpoints: RunFlow must build its config with Google's
// endpoints even when the credentials file claims attacker-controlled
// auth_uri/token_uri — the file supplies only client_id/secret.
func TestRunFlowPinsEndpoints(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs := afero.NewMemMapFs()
	credentialsPath := "/planted/credentials.json"
	require.NoError(t, afero.WriteFile(fs, credentialsPath, []byte(`{
	  "installed": {
	    "client_id": "test-client-id",
	    "client_secret": "test-client-secret",
	    "auth_uri": "https://evil.example/auth",
	    "token_uri": "https://evil.example/token",
	    "redirect_uris": ["http://localhost"]
	  }
	}`), 0o600))

	var conf *oauth2.Config
	hooks.exchangeFn = func(c *oauth2.Config, _ string) (*oauth2.Token, error) {
		conf = c
		return &oauth2.Token{AccessToken: "access-1", Expiry: time.Now().Add(time.Hour)}, nil
	}

	res := startFlow(fs, credentialsPath, nil)
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.NoError(t, (<-res).err)
	require.NotNil(t, conf)
	assert.Equal(t, google.Endpoint.AuthURL, conf.Endpoint.AuthURL)
	assert.Equal(t, google.Endpoint.TokenURL, conf.Endpoint.TokenURL)
}

func TestRunFlowExchangeError(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, credentialsPath := writeCredentialsFile(t)
	hooks.exchangeFn = func(*oauth2.Config, string) (*oauth2.Token, error) {
		return nil, errors.New("bad code")
	}

	res := startFlow(fs, credentialsPath, nil)
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	got := <-res
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "exchanging authorization code")
}

func TestRunFlowUserinfoError(t *testing.T) {
	hooks := stubFlowSeams(t)
	fs, credentialsPath := writeCredentialsFile(t)
	hooks.emailFn = func(*oauth2.Token) (string, error) {
		return "", errors.New("userinfo unreachable")
	}

	res := startFlow(fs, credentialsPath, nil)
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	got := <-res
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "userinfo unreachable")
}

func TestRunFlowPrintsURLWhenBrowserUnavailable(t *testing.T) {
	hooks := stubFlowSeams(t) // stubFlowSeams' cleanup restores the browser seam
	openBrowser = func(string) error { return errors.New("no browser") }

	fs, credentialsPath := writeCredentialsFile(t)
	res := startFlow(fs, credentialsPath, nil)
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"c"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.NoError(t, (<-res).err)
	assert.Contains(t, hooks.output.String(), "open the URL above manually",
		"the printed URL is the fallback when no browser can be opened")
	assert.Contains(t, hooks.output.String(), authURL)
}

func TestWithEmailScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   []string
	}{
		{
			name:   "appends email scope",
			scopes: []string{"scope-a"},
			want:   []string{"scope-a", ScopeUserEmail},
		},
		{
			name:   "already granted",
			scopes: []string{"scope-a", ScopeUserEmail},
			want:   []string{"scope-a", ScopeUserEmail},
		},
		{
			name:   "empty scopes",
			scopes: nil,
			want:   []string{ScopeUserEmail},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, withEmailScope(tt.scopes))
		})
	}
}
