package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// flowResult carries RunFlowWith's outcome from its goroutine to the test.
type flowResult struct {
	token *oauth2.Token
	email string
	err   error
}

// startFlow runs RunFlowWith against the GoogleOAuth profile in the
// background and returns its result channel.
func startFlow(scopes []string) chan flowResult {
	res := make(chan flowResult, 1)
	go func() {
		token, email, err := RunFlowWith(testClientCredentials, scopes, GoogleOAuth)
		res <- flowResult{token: token, email: email, err: err}
	}()
	return res
}

func TestRunFlowWith(t *testing.T) {
	hooks := stubFlowSeams(t)

	var mu sync.Mutex
	var gotCode, gotRedirect, gotVerifier string
	var gotScopes []string
	var gotToken *oauth2.Token
	hooks.exchangeFn = func(conf *oauth2.Config, code, verifier string) (*oauth2.Token, error) {
		mu.Lock()
		defer mu.Unlock()
		gotCode, gotRedirect, gotScopes = code, conf.RedirectURL, conf.Scopes
		gotVerifier = verifier
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

	res := startFlow([]string{"scope-a"})

	// Act as the browser: take the printed URL, then hit the redirect URI.
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "state-123", u.Query().Get("state"), "flow must pin a CSRF state")
	assert.Equal(t, "S256", u.Query().Get("code_challenge_method"), "PKCE must use S256")
	gotChallenge := u.Query().Get("code_challenge")
	require.Len(t, gotChallenge, 43, "S256 challenge is 43 base64url chars")
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
	// The PKCE verifier presented at the token endpoint must match the
	// challenge carried on the auth URL.
	sum := sha256.Sum256([]byte(gotVerifier))
	assert.Equal(t, gotChallenge, base64.RawURLEncoding.EncodeToString(sum[:]),
		"exchanged verifier must hash to the auth-URL code_challenge")
}

func TestRunFlowWithStateMismatch(t *testing.T) {
	hooks := stubFlowSeams(t)

	res := startFlow(nil)

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

func TestRunFlowWithExchangeError(t *testing.T) {
	hooks := stubFlowSeams(t)
	hooks.exchangeFn = func(*oauth2.Config, string, string) (*oauth2.Token, error) {
		return nil, errors.New("bad code")
	}

	res := startFlow(nil)
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

func TestRunFlowWithUserinfoError(t *testing.T) {
	hooks := stubFlowSeams(t)
	hooks.emailFn = func(*oauth2.Token) (string, error) {
		return "", errors.New("userinfo unreachable")
	}

	res := startFlow(nil)
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

func TestRunFlowWithPrintsURLWhenBrowserUnavailable(t *testing.T) {
	hooks := stubFlowSeams(t) // stubFlowSeams' cleanup restores the browser seam
	openBrowser = func(string) error { return errors.New("no browser") }

	res := startFlow(nil)
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

// TestNewStateFailsClosed: when randomness is unavailable, state generation
// must fail and the flow must abort — never fall back to a predictable state.
func TestNewStateFailsClosed(t *testing.T) {
	savedRand := randRead
	t.Cleanup(func() { randRead = savedRand })
	randRead = func([]byte) (int, error) { return 0, errors.New("entropy exhausted") }

	_, err := newState()
	require.Error(t, err)

	hooks := stubFlowSeams(t)
	newState = func() (string, error) { return "", errors.New("entropy exhausted") }

	got := <-startFlow(nil)

	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "entropy exhausted")
	assert.Nil(t, got.token)
	assert.NotRegexp(t, authURLPattern, hooks.output.String(),
		"no authorization URL may be printed when state generation fails")
}

// TestRunFlowWithAppliesDeadlines: the code exchange and userinfo fetch contexts
// must each carry a deadline, so hung endpoints cannot stall the flow.
func TestRunFlowWithAppliesDeadlines(t *testing.T) {
	hooks := stubFlowSeams(t)

	var mu sync.Mutex
	var exDeadline, emDeadline bool
	exchangeCode = func(ctx context.Context, _ *oauth2.Config, _ string, _ string) (*oauth2.Token, error) {
		d, ok := ctx.Deadline()
		mu.Lock()
		exDeadline = ok && d.After(time.Now())
		mu.Unlock()
		return &oauth2.Token{AccessToken: "access-1", Expiry: time.Now().Add(time.Hour)}, nil
	}
	fetchEmail = func(ctx context.Context, _ string, _ *oauth2.Token) (string, error) {
		d, ok := ctx.Deadline()
		mu.Lock()
		emDeadline = ok && d.After(time.Now())
		mu.Unlock()
		return "user@example.com", nil
	}

	res := startFlow(nil)
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.NoError(t, (<-res).err)
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, exDeadline, "the exchange context must carry a deadline")
	assert.True(t, emDeadline, "the userinfo context must carry a deadline")
}

// TestRunFlowWithExchangeTimesOut: a token endpoint that never answers must fail
// the flow within the exchange deadline, not hang forever.
func TestRunFlowWithExchangeTimesOut(t *testing.T) {
	hooks := stubFlowSeams(t)
	savedTimeout := networkTimeout
	networkTimeout = 100 * time.Millisecond
	t.Cleanup(func() { networkTimeout = savedTimeout })

	exchangeCode = func(ctx context.Context, _ *oauth2.Config, _, _ string) (*oauth2.Token, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	res := startFlow(nil)
	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"state-123"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	select {
	case got := <-res:
		require.Error(t, got.err)
		assert.Contains(t, got.err.Error(), "exchanging authorization code")
	case <-time.After(5 * time.Second):
		t.Fatal("the flow did not abort on a hung token endpoint")
	}
}

// TestRunFlowWithPKCEOnTheWire drives the real exchangeCode (with its
// code_verifier auth option) against a local token endpoint, proving the
// verifier presented on the wire hashes to the auth-URL code_challenge.
func TestRunFlowWithPKCEOnTheWire(t *testing.T) {
	savedConf, savedOutput, savedBrowser, savedEmail, savedState := oauthConfigFor, flowOutput, openBrowser, fetchEmail, newState
	t.Cleanup(func() {
		oauthConfigFor, flowOutput, openBrowser, fetchEmail, newState = savedConf, savedOutput, savedBrowser, savedEmail, savedState
	})

	var mu sync.Mutex
	var forms []url.Values
	tokSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		forms = append(forms, r.PostForm)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`)
	}))
	t.Cleanup(tokSrv.Close)

	oauthConfigFor = func(_ OAuthProfile, creds ClientCredentials, scopes ...string) *oauth2.Config {
		return &oauth2.Config{
			ClientID:     creds.ID,
			ClientSecret: creds.Secret,
			Endpoint:     oauth2.Endpoint{AuthURL: tokSrv.URL + "/auth", TokenURL: tokSrv.URL + "/token"},
			Scopes:       scopes,
		}
	}
	out := &syncBuffer{}
	flowOutput = out
	openBrowser = func(string) error { return nil }
	newState = func() (string, error) { return "wire-state", nil }
	fetchEmail = func(_ context.Context, _ string, _ *oauth2.Token) (string, error) { return "user@example.com", nil }

	res := startFlow(nil)

	authURL := waitAuthURL(t, out)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	callback := u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {"wire-state"}}.Encode()
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.NoError(t, (<-res).err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, forms, 1, "exactly one token request")
	form := forms[0]
	verifier := form.Get("code_verifier")
	require.Len(t, verifier, 128, "the verifier is 64 entropy bytes hex-encoded")
	sum := sha256.Sum256([]byte(verifier))
	assert.Equal(t, u.Query().Get("code_challenge"), base64.RawURLEncoding.EncodeToString(sum[:]),
		"the on-the-wire verifier must hash to the auth-URL code_challenge")
	assert.Equal(t, "test-code", form.Get("code"))
}

// TestRunFlowWithIdentityResolver: a profile carrying an IdentityResolver
// (Linear's GraphQL viewer query) resolves the account email through it
// instead of the userinfo GET.
func TestRunFlowWithIdentityResolver(t *testing.T) {
	hooks := stubFlowSeams(t)

	profile := OAuthProfile{
		Name:        "other-cli",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://provider.example/auth", TokenURL: "https://provider.example/token"},
		UserinfoURL: "https://provider.example/userinfo",
		EmailScope:  "provider-email-scope",
		IdentityResolver: func(_ context.Context, tok *oauth2.Token) (string, error) {
			assert.Equal(t, "access-1", tok.AccessToken,
				"the resolver is called with the freshly exchanged token")
			return "resolved@provider.example", nil
		},
	}
	// The userinfo path must not run when a resolver is attached.
	fetchEmail = func(context.Context, string, *oauth2.Token) (string, error) {
		return "", errors.New("userinfo GET must not run with an IdentityResolver")
	}

	res := make(chan flowResult, 1)
	go func() {
		tok, email, err := RunFlowWith(testClientCredentials, nil, profile)
		res <- flowResult{token: tok, email: email, err: err}
	}()

	authURL := waitAuthURL(t, hooks.output)
	callback := redirectCallback(t, authURL, "state-123")
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	got := <-res
	require.NoError(t, got.err)
	assert.Equal(t, "resolved@provider.example", got.email)
}

// TestRunFlowWithScopeSeparator: a profile's scope separator (Linear
// documents comma-separated scopes) joins the scope list on the
// authorization URL.
func TestRunFlowWithScopeSeparator(t *testing.T) {
	hooks := stubFlowSeams(t)

	profile := OAuthProfile{
		Name:           "other-cli",
		Endpoint:       oauth2.Endpoint{AuthURL: "https://provider.example/auth", TokenURL: "https://provider.example/token"},
		UserinfoURL:    "https://provider.example/userinfo",
		EmailScope:     "provider-email-scope",
		ScopeSeparator: ",",
	}

	res := make(chan flowResult, 1)
	go func() {
		tok, email, err := RunFlowWith(testClientCredentials, []string{"scope-a"}, profile)
		res <- flowResult{token: tok, email: email, err: err}
	}()

	authURL := waitAuthURL(t, hooks.output)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "scope-a,provider-email-scope", u.Query().Get("scope"))

	callback := redirectCallback(t, authURL, "state-123")
	resp, err := http.Get(callback)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.NoError(t, (<-res).err)
}

func TestEnsureEmailScope(t *testing.T) {
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
			assert.Equal(t, tt.want, ensureScope(tt.scopes, ScopeUserEmail))
		})
	}
}
