// Package account implements the everything-cli account subcommands: managing
// the per-account OAuth token cache and the default account.
package account

import (
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/spf13/cobra"
)

// NewCmd builds the account parent command. Every leaf inherits the root's
// persistent flags (--account, --format, --credentials, --debug).
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Google accounts and their cached OAuth tokens",
		Long: "Manage Google accounts: authorize them with the Google OAuth flow, " +
			"list them, inspect one, pick the default account, and remove them.",
	}

	cmd.AddCommand(newListCmd(cfg))
	cmd.AddCommand(newAddCmd(cfg))
	cmd.AddCommand(newGetCmd(cfg))
	cmd.AddCommand(newUseCmd(cfg))
	cmd.AddCommand(newRemoveCmd(cfg))

	return cmd
}
