package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// getAccount is the rendered shape of account get: name and provider only.
// The API key is deliberately absent from the view, so no output format can
// leak it.
type getAccount struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// newAccountGetCmd builds `granola account get`: one account's metadata.
// The API key is never printed.
func newAccountGetCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show a Granola account's metadata (the API key is never printed)",
		Example: `# Show the "work" account
everything-cli granola account get work

# Show the "work" account as JSON
everything-cli granola account get work --format json`,
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
