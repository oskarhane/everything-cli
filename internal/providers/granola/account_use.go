package granola

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
)

// newAccountUseCmd builds `granola account use`: set the provider's default
// account other commands act as when --account is not given.
func newAccountUseCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default Granola account",
		Example: `# Make the "work" account the default
everything-cli granola account use work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			if err := store.SetDefaultAccountFor(providerID, args[0]); err != nil {
				return fmt.Errorf("setting default granola account: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default account set to %s\n", args[0])
			return nil
		},
	}
}
