package account

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
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

// stubRunFlow replaces the runFlow seam for the test's lifetime so no test
// starts a real browser authorization.
func stubRunFlow(t *testing.T, fn func(credentialsPath string, scopes []string) (*oauth2.Token, string, error)) {
	t.Helper()
	saved := runFlow
	runFlow = fn
	t.Cleanup(func() { runFlow = saved })
}

// writeCredentials writes a credentials file on the in-memory FS.
func writeCredentials(t *testing.T, cfg *app.Config, path string) {
	t.Helper()
	require.NoError(t, cfg.Fs.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, afero.WriteFile(cfg.Fs, path, []byte("{}"), 0o600))
}
