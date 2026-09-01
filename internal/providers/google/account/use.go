package account

import (
	"fmt"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/cobra"
)

// newUseCmd builds account use: set the default account other commands act
// as when --account is not given.
func newUseCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default Google account",
		Example: `# Make the "work" account the default
everything-cli account use work

# Switch back to the "personal" account
everything-cli account use personal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			if err := store.SetDefaultAccount(args[0]); err != nil {
				return fmt.Errorf("setting default account: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default account set to %s\n", args[0])
			return nil
		},
	}
}
