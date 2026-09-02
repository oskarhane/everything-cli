package email

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

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

// newEmailEnv returns a hermetic command tree: an in-memory FS, a pinned
// config dir, and the provider tree mounted on a fresh root command whose
// stdout is captured. Tests never touch the real config dir. The password
// env var is blanked so a host EMAIL_PASSWORD cannot leak into a test.
func newEmailEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	t.Setenv(passwordEnvVar, "")
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

// stubCapture swaps the getenv/prompt seams for the test's lifetime so
// capture-order tests run hermetically — never against the real
// environment or a terminal.
func stubCapture(t *testing.T, getenvFn func(string) string, promptFn func() (string, error)) {
	t.Helper()
	savedGetenv, savedPrompt := getenv, prompt
	if getenvFn != nil {
		getenv = getenvFn
	}
	if promptFn != nil {
		prompt = promptFn
	}
	t.Cleanup(func() { getenv, prompt = savedGetenv, savedPrompt })
}

// seedAccount persists an email account directly through addAccount so
// read/remove tests start from a known state without a prompt. The
// password is distinctive per test so assertions can prove it never
// reaches output.
func seedAccount(t *testing.T, cfg *app.Config, name, password string) {
	t.Helper()
	_, err := addAccount(newStore(t, cfg), addOptions{
		Name:     name,
		Username: name + "@example.com",
		Password: password,
		IMAPHost: "imap.example.com",
		IMAPPort: defaultIMAPPort,
		SMTPHost: "smtp.example.com",
		SMTPPort: defaultSMTPPort,
	})
	require.NoError(t, err)
}
