package granola

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
)

// TestMain pins format auto-detection off: the host machine may run this
// suite inside an agent harness or a TTY, and neither may flip expectations.
// Tests always pass an explicit --format when the format matters.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newGranolaEnv returns a hermetic command tree: an in-memory FS, a pinned
// config dir, and the provider tree mounted on a fresh root command whose
// stdout is captured. Tests never touch the real config dir. The API-key
// env var is blanked so a host GRANOLA_API_KEY cannot leak into a test.
func newGranolaEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	t.Setenv("GRANOLA_API_KEY", "")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(Provider{}.NewCmd(cfg))
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

// stubNotes swaps the dialNotes seam for the test's lifetime so note leaves
// run against a hermetic service (an httptest-backed httpService), never
// the network or a real account.
func stubNotes(t *testing.T, svc NoteService) {
	t.Helper()
	saved := dialNotes
	dialNotes = func(context.Context, *app.Config) (NoteService, error) { return svc, nil }
	t.Cleanup(func() { dialNotes = saved })
}

// seedAccount persists a granola account directly through the strategy so
// read/remove tests start from a known state without a prompt. The key is
// distinctive per test so assertions can prove it never reaches output.
func seedAccount(t *testing.T, cfg *app.Config, name, key string) {
	t.Helper()
	_, err := strategy.Add(context.Background(), cfg.Fs, newStore(t, cfg), auth.AddOptions{Name: name, APIKey: key})
	require.NoError(t, err)
}
