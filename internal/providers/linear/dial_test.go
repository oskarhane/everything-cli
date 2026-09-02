package linear

import (
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
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

// resolveForTest runs the canonical provider-scoped resolver the way dial
// does; tests pin its linear error texts through this helper.
func resolveForTest(cfg *app.Config) (*config.Account, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, err
	}
	return auth.ResolveAccountFor(cfg, store, ID)
}

func TestResolveAccountNoAccounts(t *testing.T) {
	_, err := resolveForTest(newDialConfig(t))
	require.ErrorContains(t, err, "no linear accounts configured; run `everything-cli linear account add`")
}

func TestResolveAccountNoDefault(t *testing.T) {
	// Store.Save auto-sets the provider default on first save, so the
	// no-default state can only be constructed by writing the account file
	// directly, with no config.json holding a default.
	cfg := newDialConfig(t)
	accountJSON := `{"name":"work","provider":"linear","auth":{"api_key":"test-key-123"}}` + "\n"
	require.NoError(t, afero.WriteFile(cfg.Fs, "/config/accounts/linear/work.json", []byte(accountJSON), 0o600))

	_, err := resolveForTest(cfg)
	require.ErrorContains(t, err, "no default linear account set; run `everything-cli linear account use <name>` or pass --account")
}

func TestResolveAccountDefault(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work") // first save becomes the provider default

	acct, err := resolveForTest(cfg)
	require.NoError(t, err)
	require.Equal(t, "work", acct.Name)
}

func TestResolveAccountFlagWins(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work")
	seedAccount(t, cfg, "personal")
	cfg.Account = "personal"

	acct, err := resolveForTest(cfg)
	require.NoError(t, err)
	require.Equal(t, "personal", acct.Name)
}

func TestResolveAccountUnknown(t *testing.T) {
	cfg := newDialConfig(t)
	seedAccount(t, cfg, "work")
	cfg.Account = "ghost"

	_, err := resolveForTest(cfg)
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
	require.ErrorContains(t, err, "no linear accounts configured; run `everything-cli linear account add`")
}
