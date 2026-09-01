package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// newRemoveCmd builds account remove: delete a Linear account and its
// stored API key. Refuses without --force so an accidental removal never
// loses a working key.
func newRemoveCmd(cfg *app.Config, providerID string) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a Linear account and its stored API key",
		Example: `# Inspect what would be removed, then remove the "old" account
everything-cli linear account get old
everything-cli linear account remove old --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf(
					"refusing to remove account %q; pass --force to delete it and its stored API key",
					args[0])
			}
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			if err := store.RemoveProvider(providerID, args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed account %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false,
		"Remove the account and its stored API key (required)")
	return cmd
}
