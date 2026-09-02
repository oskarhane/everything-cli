// Package account implements the `linear account` subcommands: managing
// Linear's provider-scoped accounts (API-key or OAuth) and the provider's
// default account.
package account

import (
	"github.com/spf13/cobra"

	sharedaccount "github.com/oskarhane/everything-cli/internal/account"
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
)

// StrategyFactory builds the auth strategy account add onboards through.
// Production wires the linear provider's API-key strategy; tests substitute
// fakes so the flag/env/prompt capture paths run hermetically.
type StrategyFactory func() auth.Strategy

// NewCmd builds the linear account parent command, scoped to the provider
// ID so accounts resolve under accounts/<provider>/ only. The
// list/get/use/remove leaves come from the shared account builder; add
// stays here because it is strategy-specific. Every leaf inherits the
// root's persistent flags (--account, --format, --debug).
func NewCmd(cfg *app.Config, providerID string, newStrategy StrategyFactory) *cobra.Command {
	spec := sharedaccount.Spec{
		ProviderID:  providerID,
		DisplayName: "Linear",
		Credential:  "stored API key",
	}
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Linear accounts and their credentials",
		Long: "Manage Linear accounts: add them with a personal API key or " +
			"OAuth (--oauth), list them, inspect one, pick the default account, " +
			"and remove them.",
	}

	cmd.AddCommand(sharedaccount.NewListCmd(cfg, spec))
	cmd.AddCommand(newAddCmd(cfg, providerID, newStrategy))
	cmd.AddCommand(sharedaccount.NewGetCmd(cfg, spec))
	cmd.AddCommand(sharedaccount.NewUseCmd(cfg, spec))
	cmd.AddCommand(sharedaccount.NewRemoveCmd(cfg, spec))

	return cmd
}
