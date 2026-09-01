package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// listAccount is one rendered row of account list. The provider's default
// account is marked with default: true.
type listAccount struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

// newAccountListCmd builds `granola account list`: every configured granola
// account, with the default marked. API keys are never printed.
func newAccountListCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured Granola accounts",
		Example: `# List all configured Granola accounts
everything-cli granola account list

# List accounts as JSON; the default account carries "default": true
everything-cli granola account list --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			accounts, err := store.ListProvider(providerID)
			if err != nil {
				return err
			}
			def, err := store.DefaultAccountFor(providerID)
			if err != nil {
				return err
			}

			rows := make([]listAccount, 0, len(accounts))
			for _, a := range accounts {
				rows = append(rows, listAccount{Name: a.Name, Default: a.Name == def})
			}

			tableRows := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				marker := ""
				if r.Default {
					marker = "(default)"
				}
				tableRows = append(tableRows, map[string]any{
					"name":    r.Name,
					"default": marker,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "default"}, rows, tableRows)
			return nil
		},
	}
}
