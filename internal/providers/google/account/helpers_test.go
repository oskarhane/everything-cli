package account

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// TestMain pins format auto-detection off: the host machine may run this
// suite inside an agent harness or a TTY, and neither may flip expectations.
// Tests always pass an explicit --format when the format matters.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newAccountEnv returns a hermetic account tree: an in-memory FS, a pinned
// config dir, and the account tree mounted on a fresh root command whose
// stdout is captured. Tests never touch the real ~/.config/google-cli.
func newAccountEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(NewCmd(cfg))
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(io.Discard)
	return cfg, root, out
}

// execute runs the command tree with args and returns the captured stdout
// and the command's error. Cobra's usage/error output goes to io.Discard so
// out holds only command output.
func execute(t *testing.T, root *cobra.Command, out *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// newStore returns the store the commands build: the injected in-memory FS
// with the same config-dir resolution as production.
func newStore(t *testing.T, cfg *app.Config) *config.Store {
	t.Helper()
	store, err := config.NewStore(cfg.Fs, "")
	require.NoError(t, err)
	return store
}

// seedAccount persists an account directly in the store (bypassing the OAuth
// flow) so read/remove tests start from a known state.
func seedAccount(t *testing.T, cfg *app.Config, name, email string) {
	t.Helper()
	_, err := auth.SaveAccount(newStore(t, cfg), name, email,
		[]string{auth.ScopeUserEmail}, testToken(name))
	require.NoError(t, err)
}

// testToken builds a token whose secret values are distinctive per account,
// so tests can assert they never reach any output format.
func testToken(name string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "secret-access-" + name,
		RefreshToken: "secret-refresh-" + name,
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

// stubAddStrategy replaces the newAddStrategy seam for the test's lifetime
// with a fake strategy whose Add runs fn, so no test ever starts a real
// browser authorization.
func stubAddStrategy(t *testing.T, fn func(creds auth.ClientCredentials, scopes []string) (*oauth2.Token, string, error)) {
	t.Helper()
	saved := newAddStrategy
	newAddStrategy = func(*config.Store, auth.ClientCredentials) auth.Strategy {
		return fakeStrategy{fn: fn}
	}
	t.Cleanup(func() { newAddStrategy = saved })
}

// fakeStrategy is a test auth.Strategy whose Add runs the stubbed flow and
// persists through the real provider-scoped store, mirroring the production
// OAuth strategy without a browser.
type fakeStrategy struct {
	fn func(creds auth.ClientCredentials, scopes []string) (*oauth2.Token, string, error)
}

func (f fakeStrategy) Add(_ context.Context, _ afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = auth.GoogleOAuth.DefaultScopes
	}
	tok, email, err := f.fn(opts.Credentials, scopes)
	if err != nil {
		return nil, err
	}
	saved, err := auth.SaveAccount(store, opts.Name, email, scopes, tok)
	if err != nil {
		return nil, err
	}
	return store.Get(saved)
}

func (f fakeStrategy) Client(context.Context, *config.Account) (*http.Client, error) {
	return nil, errors.New("fakeStrategy has no client")
}

// writeCredentials writes a valid installed-app credentials file on the
// in-memory FS.
func writeCredentials(t *testing.T, cfg *app.Config, path string) {
	t.Helper()
	writeCredentialsWithID(t, cfg, path, "test-client-id")
}

// writeCredentialsWithID is writeCredentials with a distinctive client ID,
// so tests can tell which resolved file was parsed.
func writeCredentialsWithID(t *testing.T, cfg *app.Config, path, clientID string) {
	t.Helper()
	doc := `{"installed":{"client_id":` + strconv.Quote(clientID) +
		`,"client_secret":"test-client-secret","redirect_uris":["http://localhost"]}}`
	require.NoError(t, cfg.Fs.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, afero.WriteFile(cfg.Fs, path, []byte(doc), 0o600))
}
