package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// NewRemoveCmd builds account remove for the provider described by spec:
// delete an account and its credential. Refuses without --force so an
// accidental removal never loses a working credential. Removing the
// provider's default promotes another account of that provider (store
// policy); the new default is announced so the change is never silent.
func NewRemoveCmd(cfg *app.Config, spec Spec) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a " + spec.DisplayName + " account and its " + spec.Credential,
		Example: `# Inspect what would be removed, then remove the "old" account
everything-cli ` + spec.ProviderID + ` account get old
everything-cli ` + spec.ProviderID + ` account remove old --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf(
					"refusing to remove account %q; pass --force to delete it and its %s",
					args[0], spec.Credential)
			}
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			def, err := store.DefaultAccountFor(spec.ProviderID)
			if err != nil {
				return err
			}
			if err := store.RemoveProvider(spec.ProviderID, args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed account %s\n", args[0])
			if def != args[0] {
				return nil
			}
			promoted, err := store.DefaultAccountFor(spec.ProviderID)
			if err != nil {
				return err
			}
			if promoted != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default account is now %s\n", promoted)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false,
		"Remove the account and its "+spec.Credential+" (required)")
	return cmd
}
