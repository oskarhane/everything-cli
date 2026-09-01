// Package google wires Google as a provider of the CLI: the Gmail,
// Calendar, Drive, Docs, Sheets, Slides and YouTube resource trees plus the
// provider-scoped account subtree, behind the provider.Provider contract.
// Registration happens at init time; main.go imports this package for its
// side effect.
package google

import (
	"fmt"
	"sync"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/provider"
	"github.com/oskarhane/google-cli/internal/providers/google/account"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar"
	"github.com/oskarhane/google-cli/internal/providers/google/docs"
	"github.com/oskarhane/google-cli/internal/providers/google/drive"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail"
	"github.com/oskarhane/google-cli/internal/providers/google/sheets"
	"github.com/oskarhane/google-cli/internal/providers/google/slides"
	"github.com/oskarhane/google-cli/internal/providers/google/youtube"
)

// Provider is Google's provider.Provider implementation. It is a value
// type: construction does no I/O, so init-time registration is safe.
type Provider struct{}

// Compile-time proof that Provider satisfies the registry contract.
var _ provider.Provider = Provider{}

// init self-registers the provider; the root command discovers it through
// provider.List.
func init() {
	provider.Register(Provider{})
}

// ID returns the provider identifier — also the store's per-provider
// account directory.
func (Provider) ID() string { return config.ProviderGoogle }

// strategy is Google's auth strategy, built lazily on first use: a Strategy
// needs the account store, and resolving the store is I/O that must not
// happen at init time. The panic on store failure mirrors the registry's
// duplicate-ID panic: a config dir that cannot resolve is a startup
// misconfiguration, not a runtime condition.
var (
	strategyMu sync.Mutex
	strategy   *Strategy
)

// Auth returns Google's auth strategy: the installed-app OAuth flow behind
// the auth.Strategy seam (see auth.go).
func (Provider) Auth() auth.Strategy {
	strategyMu.Lock()
	defer strategyMu.Unlock()
	if strategy == nil {
		fs := afero.NewOsFs()
		store, err := config.NewStore(fs, "")
		if err != nil {
			panic(fmt.Sprintf("google: opening account store for auth strategy: %v", err))
		}
		strategy = NewStrategy(fs, store, "")
	}
	return strategy
}

// NewCmd builds the `google` command tree: every Google resource subtree
// plus the provider-scoped account subtree. The --credentials flag is
// Google-specific, so it lives here as a persistent flag (inherited by
// every Google leaf) rather than on the root.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Interact with Google services (Gmail, Calendar, Drive, Docs, Sheets, Slides, YouTube)",
	}
	cmd.PersistentFlags().StringVar(&cfg.Credentials, "credentials", "",
		"Path to OAuth app credentials JSON (empty = auto-resolve)")
	cmd.AddCommand(
		account.NewCmd(cfg),
		gmail.NewCmd(cfg),
		calendar.NewCmd(cfg),
		drive.NewCmd(cfg),
		docs.NewCmd(cfg),
		sheets.NewCmd(cfg),
		slides.NewCmd(cfg),
		youtube.NewCmd(cfg),
	)
	return cmd
}
