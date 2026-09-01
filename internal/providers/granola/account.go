package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newAccountCmd builds the provider-scoped `granola account` parent. Every
// leaf lives in its own file, one AddCommand line each. API keys are
// secrets: no leaf ever prints one.
func newAccountCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Granola accounts and their API keys",
	}
	cmd.AddCommand(newAccountAddCmd(cfg))
	cmd.AddCommand(newAccountListCmd(cfg))
	cmd.AddCommand(newAccountGetCmd(cfg))
	cmd.AddCommand(newAccountRemoveCmd(cfg))
	cmd.AddCommand(newAccountUseCmd(cfg))
	return cmd
}
