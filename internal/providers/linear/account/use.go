package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// newUseCmd builds account use: set the default Linear account other
// linear commands act as when --account is not given.
func newUseCmd(cfg *app.Config, providerID string) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default Linear account",
		Example: `# Make the "work" account the default
everything-cli linear account use work

# Switch back to the "personal" account
everything-cli linear account use personal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			if err := store.SetDefaultAccountFor(providerID, args[0]); err != nil {
				return fmt.Errorf("setting default account: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default account set to %s\n", args[0])
			return nil
		},
	}
}
