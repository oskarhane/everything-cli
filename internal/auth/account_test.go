package auth

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// TestResolveAccountFor pins the canonical provider-scoped resolver: flag
// wins over the provider default, and both actionable error texts name the
// provider for every registered provider.
func TestResolveAccountFor(t *testing.T) {
	providers := []string{"google", "linear", "granola"}

	seedProviderAccount := func(t *testing.T, store *config.Store, provider, name string) {
		t.Helper()
		require.NoError(t, store.Save(&config.Account{Name: name, Provider: provider}))
	}

	t.Run("flag wins over the provider default", func(t *testing.T) {
		for _, provider := range providers {
			store := newTestStore(t)
			seedProviderAccount(t, store, provider, "work") // first save becomes the default
			seedProviderAccount(t, store, provider, "personal")

			acct, err := ResolveAccountFor(&app.Config{Account: "personal"}, store, provider)
			require.NoError(t, err, provider)
			require.Equal(t, "personal", acct.Name, provider)
			require.Equal(t, provider, acct.Provider, provider)
		}
	})

	t.Run("falls back to the provider default", func(t *testing.T) {
		for _, provider := range providers {
			store := newTestStore(t)
			seedProviderAccount(t, store, provider, "work") // first save becomes the default

			acct, err := ResolveAccountFor(&app.Config{}, store, provider)
			require.NoError(t, err, provider)
			require.Equal(t, "work", acct.Name, provider)
		}
	})

	t.Run("no accounts configured", func(t *testing.T) {
		for _, provider := range providers {
			_, err := ResolveAccountFor(&app.Config{}, newTestStore(t), provider)
			require.EqualError(t, err, fmt.Sprintf(
				"no %s accounts configured; run `everything-cli %s account add`", provider, provider), provider)
		}
	})

	t.Run("accounts exist but no default set", func(t *testing.T) {
		// Store.Save auto-sets the provider default on first save, so the
		// no-default state can only be constructed by writing the account
		// file directly, with no config.json holding a default.
		for _, provider := range providers {
			fs := afero.NewMemMapFs()
			accountJSON := fmt.Sprintf(`{"name":"work","provider":%q}`, provider) + "\n"
			require.NoError(t, afero.WriteFile(fs, "/config/accounts/"+provider+"/work.json", []byte(accountJSON), 0o600))
			store, err := config.NewStore(fs, "/config")
			require.NoError(t, err)

			_, err = ResolveAccountFor(&app.Config{}, store, provider)
			require.EqualError(t, err, fmt.Sprintf(
				"no default %s account set; run `everything-cli %s account use <name>` or pass --account", provider, provider), provider)
		}
	})

	t.Run("unknown flag value names the provider", func(t *testing.T) {
		for _, provider := range providers {
			store := newTestStore(t)
			seedProviderAccount(t, store, provider, "work")

			_, err := ResolveAccountFor(&app.Config{Account: "ghost"}, store, provider)
			require.ErrorContains(t, err, fmt.Sprintf(`resolving %s account "ghost"`, provider), provider)
		}
	})
}
