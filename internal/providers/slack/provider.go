// Package slack wires Slack (slack.com/api) as a provider of the CLI: the
// Web API auth strategy (Authorization: Bearer xoxp-<token>) behind the
// provider.Provider contract, plus the shared HTTP service and output types
// every slack resource tree consumes. Registration happens at init time; the
// root command discovers the provider once the side-effect import is added
// to main.go.
package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/provider"
)

// providerID is the registry key and the store's per-provider directory.
const providerID = "slack"

// Provider is Slack's provider.Provider implementation. It is a value type:
// construction does no I/O, so init-time registration is safe.
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

// NewCmd builds the `slack` command tree. The account subtree and the
// resource subtrees (search, channel, thread, user) are wired by their own
// nodes; the parent ships bare today.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "slack",
		Short: "Read Slack workspaces via the Slack Web API",
	}
}
