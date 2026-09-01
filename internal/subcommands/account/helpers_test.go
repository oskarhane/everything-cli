package account

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
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
// stdout is captured. Tests never touch the real config dir.
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

// seedAccount persists a provider account directly in the store so list
// tests start from a known state. Save auto-manages the provider default:
// the first account seeded per provider becomes it.
func seedAccount(t *testing.T, cfg *app.Config, provider, name, email string) {
	t.Helper()
	acct := &config.Account{Name: name, Provider: provider}
	if provider == config.ProviderGoogle {
		acct.Email = email
	} else {
		acct.Identity = map[string]string{"email": email}
	}
	require.NoError(t, newStore(t, cfg).Save(acct))
}
