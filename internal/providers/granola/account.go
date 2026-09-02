package granola

import (
	"github.com/spf13/cobra"

	sharedaccount "github.com/oskarhane/everything-cli/internal/account"
	"github.com/oskarhane/everything-cli/internal/app"
)

// accountSpec scopes the shared account leaves to the granola provider:
// key-based accounts, and remove deletes the stored API key.
var accountSpec = sharedaccount.Spec{
	ProviderID:  providerID,
	DisplayName: "Granola",
	Credential:  "stored API key",
}

// newAccountCmd builds the provider-scoped `granola account` parent. The
// list/get/use/remove leaves come from the shared account builder; add
// stays here because it is strategy-specific. API keys are secrets: no leaf
// ever prints one.
func newAccountCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Granola accounts and their API keys",
	}
	cmd.AddCommand(newAccountAddCmd(cfg))
	cmd.AddCommand(sharedaccount.NewListCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewGetCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewRemoveCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewUseCmd(cfg, accountSpec))
	return cmd
}
