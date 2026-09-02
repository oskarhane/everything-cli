// Package account implements the top-level, read-only cross-provider
// account command: `account list` aggregates every provider's accounts so
// agents can discover what accounts exist anywhere in one call. Account
// management (add/use/get/remove) is provider-scoped and lives under each
// provider command (`google account add`, ...); bare `account <verb>`
// invocations are redirected there by the back-compat shim in main.
package account

import (
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/spf13/cobra"
)

// NewCmd builds the top-level account parent command. It is deliberately
// read-only: the only leaf is the cross-provider list aggregate.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "List accounts across all providers",
		Long: "Account discovery across every configured provider. Account " +
			"management is provider-scoped: run `<provider> account add|use|get|remove` " +
			"(e.g. `everything-cli google account add work`).",
	}

	cmd.AddCommand(newListCmd(cfg))

	return cmd
}
