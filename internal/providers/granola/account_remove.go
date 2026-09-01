package granola

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
)

// newAccountRemoveCmd builds `granola account remove`: delete an account
// and its stored API key. Refuses without --force so an accidental removal
// never loses a working credential. Removing the provider's default
// auto-promotes another account (store policy).
func newAccountRemoveCmd(cfg *app.Config) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a Granola account and its stored API key",
		Example: `# Inspect what would be removed, then remove the "old" account
everything-cli granola account get old
everything-cli granola account remove old --force`,
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
