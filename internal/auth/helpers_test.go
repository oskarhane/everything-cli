package auth

import (
	"bytes"
	"context"
	"net"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// authURLPattern matches the first http(s) URL in the flow output.
var authURLPattern = regexp.MustCompile(`https?://\S+`)

// newTestStore returns a hermetic accounts store on an in-memory FS: tests
// must never read or write the real ~/.config/google-cli.
func newTestStore(t *testing.T) *config.Store {
	t.Helper()
	store, err := config.NewStore(afero.NewMemMapFs(), "/config")
	require.NoError(t, err)
	return store
}

// installedAppCredentials is a minimal valid installed-app credentials file.
const installedAppCredentials = `{
  "installed": {
    "client_id": "test-client-id",
    "client_secret": "test-client-secret",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost"]
  }
}`

// writeCredentialsFile writes the test credentials JSON to an in-memory FS
// (never a real credentials location) and returns the FS and the path.
func writeCredentialsFile(t *testing.T) (afero.Fs, string) {
	t.Helper()
	fs := afero.NewMemMapFs()
	path := "/config/credentials.json"
	require.NoError(t, afero.WriteFile(fs, path, []byte(installedAppCredentials), 0o600))
	return fs, path
}

// testClientCredentials is the ClientCredentials shape of
// installedAppCredentials, for tests that bypass the file.
var testClientCredentials = ClientCredentials{ID: "test-client-id", Secret: "test-client-secret"}

// syncBuffer is a goroutine-safe bytes.Buffer: RunFlow writes the
// authorization URL from its own goroutine while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// stubFlowSeams replaces all RunFlow seams for the test's lifetime with
// hermetic defaults: no browser, fixed state, and stubbed exchange/userinfo.
type flowHooks struct {
	output     *syncBuffer
	exchangeFn func(conf *oauth2.Config, code, verifier string) (*oauth2.Token, error)
	emailFn    func(tok *oauth2.Token) (string, error)
}

func stubFlowSeams(t *testing.T) *flowHooks {
	t.Helper()
	h := &flowHooks{output: &syncBuffer{}}
	savedOutput, savedBrowser, savedListen, savedState := flowOutput, openBrowser, listenLoopback, newState
	savedExchange, savedEmail, savedConf := exchangeCode, fetchEmail, oauthConfigFor
	savedRandRead := randRead
	t.Cleanup(func() {
		flowOutput, openBrowser, listenLoopback, newState = savedOutput, savedBrowser, savedListen, savedState
		exchangeCode, fetchEmail, oauthConfigFor = savedExchange, savedEmail, savedConf
		randRead = savedRandRead
	})

	flowOutput = h.output
	openBrowser = func(string) error { return nil }
	listenLoopback = func() (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	newState = func() (string, error) { return "state-123", nil }
	exchangeCode = func(_ context.Context, conf *oauth2.Config, code, verifier string) (*oauth2.Token, error) {
		if h.exchangeFn != nil {
			return h.exchangeFn(conf, code, verifier)
		}
		return &oauth2.Token{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}, nil
	}
	fetchEmail = func(_ context.Context, _ string, tok *oauth2.Token) (string, error) {
		if h.emailFn != nil {
			return h.emailFn(tok)
		}
		return "user@example.com", nil
	}
	return h
}

// waitAuthURL waits for RunFlow to print the authorization URL and returns it.
func waitAuthURL(t *testing.T, out *syncBuffer) string {
	t.Helper()
	require.Eventually(t, func() bool {
		return authURLPattern.FindString(out.String()) != ""
	}, 5*time.Second, 10*time.Millisecond, "RunFlow should print an authorization URL")
	return authURLPattern.FindString(out.String())
}

// redirectCallback builds the loopback callback URL for a printed
// authorization URL, carrying the given state.
func redirectCallback(t *testing.T, authURL, state string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	return u.Query().Get("redirect_uri") + "?" +
		url.Values{"code": {"test-code"}, "state": {state}}.Encode()
}
