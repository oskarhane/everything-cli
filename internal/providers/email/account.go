package email

import (
	"github.com/spf13/cobra"

	sharedaccount "github.com/oskarhane/everything-cli/internal/account"
	"github.com/oskarhane/everything-cli/internal/app"
)

// accountSpec scopes the shared account leaves to the email provider:
// password-based accounts, and remove deletes the stored credentials.
var accountSpec = sharedaccount.Spec{
	ProviderID:  providerID,
	DisplayName: "Email",
	Credential:  "stored password",
}

// newAccountCmd builds the provider-scoped `email account` parent. The
// list/get/use/remove leaves come from the shared account builder; add
// stays here because it is credential-specific. Passwords are secrets: no
// leaf ever prints one.
func newAccountCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage email accounts and their IMAP/SMTP credentials",
	}
	cmd.AddCommand(newAccountAddCmd(cfg))
	cmd.AddCommand(sharedaccount.NewListCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewGetCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewRemoveCmd(cfg, accountSpec))
	cmd.AddCommand(sharedaccount.NewUseCmd(cfg, accountSpec))
	return cmd
}
