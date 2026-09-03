// Package email wires regular IMAP/SMTP mail as a provider of the CLI: a
// provider-scoped `account` subtree whose accounts hold the server
// endpoints and a username/password credential in the opaque
// config.Account.Auth JSON field. Registration happens at init time;
// main.go imports this package for its side effect. The auth.Strategy
// seam is HTTP-client-shaped and does not fit IMAP/SMTP, so the provider
// does not implement it (the registry only requires ID() + NewCmd()).
package email

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/provider"
)

// providerID is the registry key and the store's per-provider directory.
const providerID = "email"

// Provider is email's provider.Provider implementation. It is a value
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

// NewCmd builds the `email` command tree: the provider-scoped account
// subtree plus the mailbox and message resource trees. Every leaf lives in
// its own file, one AddCommand line each.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email",
		Short: "Regular email accounts over IMAP and SMTP",
	}
	cmd.AddCommand(newAccountCmd(cfg))
	cmd.AddCommand(newMailboxCmd(cfg))
	cmd.AddCommand(newMessageCmd(cfg))
	return cmd
}
