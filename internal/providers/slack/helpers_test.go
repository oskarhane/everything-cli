package slack

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// TestMain pins format auto-detection off: the host machine may run this
// suite inside an agent harness or a TTY, and neither may flip expectations.
// Tests always pass an explicit --format when the format matters.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newSlackEnv returns a hermetic command tree: an in-memory FS, a pinned
// config dir, and the provider tree mounted on a fresh root command whose
// stdout is captured. Tests never touch the real config dir. The API-key env
// var is blanked so a host SLACK_API_KEY cannot leak into a test.
func newSlackEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	t.Setenv("SLACK_API_KEY", "")
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

// stubDial swaps the dialSlack seam for the test's lifetime so resource
// leaves run against a hermetic service (an httptest-backed httpService),
// never the network or a real account.
func stubDial(t *testing.T, svc *httpService) {
	t.Helper()
	saved := dialSlack
	dialSlack = func(context.Context, *app.Config) (*httpService, error) { return svc, nil }
	t.Cleanup(func() { dialSlack = saved })
}
