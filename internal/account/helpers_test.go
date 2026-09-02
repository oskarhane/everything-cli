package account

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// googleSpec exercises the identity variant: email/scopes/token_expiry
// columns and a cached-token credential. linearSpec exercises the plain
// key-based variant.
var (
	googleSpec = Spec{ProviderID: "google", DisplayName: "Google", Identity: true, Credential: "cached token"}
	linearSpec = Spec{ProviderID: "linear", DisplayName: "Linear", Credential: "stored API key"}
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
// config dir, and the spec's shared leaves mounted under an account parent
// on a fresh root command whose stdout is captured. Tests never touch the
// real config dir.
func newAccountEnv(t *testing.T, spec Spec) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	parent := &cobra.Command{Use: "account"}
	parent.AddCommand(NewListCmd(cfg, spec))
	parent.AddCommand(NewGetCmd(cfg, spec))
	parent.AddCommand(NewUseCmd(cfg, spec))
	parent.AddCommand(NewRemoveCmd(cfg, spec))
	root.AddCommand(parent)
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

// seedGoogleAccount persists an identity-carrying account directly in the
// store (bypassing the OAuth flow) so read/remove tests start from a known
// state.
func seedGoogleAccount(t *testing.T, cfg *app.Config, name, email string) {
	t.Helper()
	_, err := auth.SaveAccount(newStore(t, cfg), name, email,
		[]string{auth.ScopeUserEmail}, testToken(name))
	require.NoError(t, err)
}

// seedKeyAccount persists a key-based provider account directly in the
// store, bypassing the capture flow.
func seedKeyAccount(t *testing.T, cfg *app.Config, provider, name, key string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"api_key": key})
	require.NoError(t, err)
	require.NoError(t, newStore(t, cfg).Save(&config.Account{
		Name:     name,
		Provider: provider,
		Auth:     payload,
	}))
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
