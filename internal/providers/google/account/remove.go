package account

import (
	"fmt"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/cobra"
)

// newRemoveCmd builds account remove: delete an account and its cached
// token. Refuses without --force so an accidental removal never loses a
// working authorization.
func newRemoveCmd(cfg *app.Config) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a Google account and its cached token",
		Example: `# Inspect what would be removed, then remove the "old" account
everything-cli google account get old
everything-cli google account remove old --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf(
					"refusing to remove account %q; pass --force to delete it and its cached token",
					args[0])
			}
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			if err := store.Remove(args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed account %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false,
		"Remove the account and its cached token (required)")
	return cmd
}
