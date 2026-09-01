package account

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
)

// getAccount is the rendered shape of account get: account metadata only.
// The API key is deliberately absent from the view, so no output format can
// leak it.
type getAccount struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// newGetCmd builds account get: one account's metadata. The API key is
// never printed.
func newGetCmd(cfg *app.Config, providerID string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show a Linear account's metadata",
		Example: `# Show the "work" account
google-cli linear account get work

# Show the "work" account as JSON (the API key is never printed)
google-cli linear account get work --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			a, err := store.GetProvider(providerID, args[0])
			if err != nil {
				return err
			}

			view := getAccount{Name: a.Name, Provider: a.Provider}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "provider"}, view,
				[]map[string]any{{"name": view.Name, "provider": view.Provider}})
			return nil
		},
	}
}
