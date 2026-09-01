package linear

import (
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
)

// newDialConfig returns a config on an in-memory FS with a pinned config
// dir: dial tests never touch the real ~/.config tree.
func newDialConfig(t *testing.T) *app.Config {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	return &app.Config{Fs: afero.NewMemMapFs()}
}

// seedAccount persists a linear account with a fake key directly in the
// store, bypassing the capture flow.
func seedAccount(t *testing.T, cfg *app.Config, name string) {
	t.Helper()
	store, err := config.NewStore(cfg.Fs, "")
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{
		Name:     name,
		Provider: ID,
		Auth:     json.RawMessage(`{"api_key":"test-key-123"}`),
	}))
}

func TestResolveAccountNoAccounts(t *testing.T) {
	_, err := resolveAccount(newDialConfig(t))
	require.ErrorContains(t, err, "no linear accounts configured; run `linear account add`")
}

func TestResolveAccountDefault(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work") // first save becomes the provider default

	acct, err := resolveAccount(cfg)
	require.NoError(t, err)
	require.Equal(t, "work", acct.Name)
}

func TestResolveAccountFlagWins(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work")
	seedAccount(t, cfg, "personal")
	cfg.Account = "personal"

	acct, err := resolveAccount(cfg)
	require.NoError(t, err)
	require.Equal(t, "personal", acct.Name)
}

func TestResolveAccountUnknown(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work")
	cfg.Account = "ghost"

	_, err := resolveAccount(cfg)
	require.ErrorContains(t, err, `resolving linear account "ghost"`)
}

func TestDialBuildsServiceForDefaultAccount(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work")

	svc, err := dial(t.Context(), cfg)
	require.NoError(t, err)
	require.NotNil(t, svc)
}

func TestDialFailsWithoutAccounts(t *testing.T) {
	_, err := dial(t.Context(), newDialConfig(t))
	require.ErrorContains(t, err, "no linear accounts configured; run `linear account add`")
}
