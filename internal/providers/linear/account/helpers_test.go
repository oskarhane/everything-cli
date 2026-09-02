package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/auth/apikey"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// providerID is the provider these tests scope accounts to.
const testProviderID = "linear"

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
// stdout is captured. Tests never touch the real ~/.config tree.
func newAccountEnv(t *testing.T, factory StrategyFactory) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(NewCmd(cfg, testProviderID, factory))
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(io.Discard)
	return cfg, root, out
}

// execute runs the command tree with args and returns the captured stdout
// and the command's error.
func execute(t *testing.T, root *cobra.Command, out *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// newStore returns the store the commands build.
func newStore(t *testing.T, cfg *app.Config) *config.Store {
	t.Helper()
	store, err := config.NewStore(cfg.Fs, "")
	require.NoError(t, err)
	return store
}

// realStrategy builds the production API-key strategy: flag, then
// LINEAR_API_KEY, then a hidden prompt.
func realStrategy(t *testing.T) StrategyFactory {
	t.Helper()
	return func() auth.Strategy {
		s, err := apikey.New(apikey.Config{
			Provider:     testProviderID,
			HeaderName:   "Authorization",
			HeaderFormat: "%s",
			EnvVar:       "LINEAR_API_KEY",
		})
		require.NoError(t, err)
		return s
	}
}

// seedAccount persists an account with a fake key directly in the store,
// bypassing the capture flow.
func seedAccount(t *testing.T, cfg *app.Config, name, key string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"api_key": key})
	require.NoError(t, err)
	require.NoError(t, newStore(t, cfg).Save(&config.Account{
		Name:     name,
		Provider: testProviderID,
		Auth:     payload,
	}))
}

// storedKey returns the API key persisted for the named account.
func storedKey(t *testing.T, cfg *app.Config, name string) string {
	t.Helper()
	acct, err := newStore(t, cfg).GetProvider(testProviderID, name)
	require.NoError(t, err)
	var payload struct {
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal(acct.Auth, &payload))
	return payload.APIKey
}

// fakePromptStrategy simulates the hidden-prompt capture path: it records
// the Add options, requires the flag and env sources to be empty, and
// persists the "prompted" key exactly like the real strategy does.
type fakePromptStrategy struct {
	key string
	got auth.AddOptions
}

func (f *fakePromptStrategy) Add(_ context.Context, _ afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
	f.got = opts
	if opts.APIKey != "" {
		return nil, errors.New("prompt path must not see a flag key")
	}
	if os.Getenv("LINEAR_API_KEY") != "" {
		return nil, errors.New("prompt path must not see an env key")
	}
	auth.RegisterSecret(f.key)
	payload, err := json.Marshal(map[string]string{"api_key": f.key})
	if err != nil {
		return nil, err
	}
	acct := &config.Account{Name: opts.Name, Provider: testProviderID, Auth: payload}
	if err := store.Save(acct); err != nil {
		return nil, err
	}
	return store.GetProvider(testProviderID, acct.Name)
}

func (f *fakePromptStrategy) Client(context.Context, *config.Account) (*http.Client, error) {
	return nil, errors.New("fakePromptStrategy has no client")
}
