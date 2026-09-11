package slack

import (
	"github.com/spf13/cobra"

	sharedaccount "github.com/oskarhane/everything-cli/internal/account"
	"github.com/oskarhane/everything-cli/internal/app"
)

// accountSpec scopes the shared account leaves to the slack provider:
// key-based accounts whose credential is a stored xoxp user token.
var accountSpec = sharedaccount.Spec{
	ProviderID:  providerID,
	DisplayName: "Slack",
	Credential:  "stored user token",
}

// newAccountCmd builds the provider-scoped `slack account` parent. The
// list/get/use/remove leaves come from the shared account builder; add
// stays here because it captures Slack's user token through the API-key
// strategy (auth.test validation included). Tokens are secrets: no leaf ever
// prints one.
func newAccountCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Slack accounts and their user tokens",
	}
	cmd.AddCommand(newAccountAddCmd(cfg))
	cmd.AddCommand(sharedaccount.NewListCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewGetCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewRemoveCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewUseCmd(cfg, accountSpec))
	return cmd
}
