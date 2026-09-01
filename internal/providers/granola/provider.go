// Package granola wires Granola (public-api.granola.ai) as a provider of
// the CLI: the `note` resource tree (list/get), a provider-scoped `account`
// subtree, and the API-key auth strategy (Authorization: Bearer grn_<key>)
// behind the provider.Provider contract. Registration happens at init time;
// main.go imports this package for its side effect.
package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/provider"
)

// providerID is the registry key and the store's per-provider directory.
const providerID = "granola"

// Provider is Granola's provider.Provider implementation. It is a value
// type: construction does no I/O, so init-time registration is safe.
type Provider struct{}

// Compile-time proof that Provider satisfies the registry contract.
var _ provider.Provider = Provider{}

// init self-registers the provider; the root command discovers it through
// provider.List.
func init() {
	provider.Register(Provider{})
}

// ID returns the provider identifier.
func (Provider) ID() string { return providerID }

// Auth returns the API-key strategy: Bearer grn_<key> in the Authorization
// header, captured via --api-key / GRANOLA_API_KEY / hidden prompt.
func (Provider) Auth() auth.Strategy { return strategy }

// NewCmd builds the `granola` command tree: the note resource subtree and
// the provider-scoped account subtree. Every leaf lives in its own file,
// one AddCommand line each.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "granola",
		Short: "Read Granola notes via the Granola public API",
	}
	cmd.AddCommand(newNoteCmd(cfg))
	cmd.AddCommand(newAccountCmd(cfg))
	return cmd
}
