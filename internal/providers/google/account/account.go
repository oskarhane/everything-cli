// Package account implements the everything-cli account subcommands: managing
// the per-account OAuth token cache and the default account.
package account

import (
	"github.com/spf13/cobra"

	sharedaccount "github.com/oskarhane/everything-cli/internal/account"
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// spec scopes the shared account leaves to the google provider: accounts
// carry an email identity, and remove deletes the cached OAuth token.
var spec = sharedaccount.Spec{
	ProviderID:  config.ProviderGoogle,
	DisplayName: "Google",
	Identity:    true,
	Credential:  "cached token",
}

// NewCmd builds the account parent command. The list/get/use/remove leaves
// come from the shared account builder; add stays here because it is
// Google-OAuth-specific. Every leaf inherits the root's persistent flags
// (--account, --format, --credentials, --debug).
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Google accounts and their cached OAuth tokens",
		Long: "Manage Google accounts: authorize them with the Google OAuth flow, " +
			"list them, inspect one, pick the default account, and remove them.",
	}

	cmd.AddCommand(sharedaccount.NewListCmd(cfg, spec))
	cmd.AddCommand(newAddCmd(cfg))
	cmd.AddCommand(sharedaccount.NewGetCmd(cfg, spec))
	cmd.AddCommand(sharedaccount.NewUseCmd(cfg, spec))
	cmd.AddCommand(sharedaccount.NewRemoveCmd(cfg, spec))

	return cmd
}
