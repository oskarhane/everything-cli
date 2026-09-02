package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// NewUseCmd builds account use for the provider described by spec: set the
// provider's default account other commands act as when --account is not
// given.
func NewUseCmd(cfg *app.Config, spec Spec) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default " + spec.DisplayName + " account",
		Example: `# Make the "work" account the default
everything-cli ` + spec.ProviderID + ` account use work

# Switch back to the "personal" account
everything-cli ` + spec.ProviderID + ` account use personal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			if err := store.SetDefaultAccountFor(spec.ProviderID, args[0]); err != nil {
				return fmt.Errorf("setting default account: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default account set to %s\n", args[0])
			return nil
		},
	}
}
