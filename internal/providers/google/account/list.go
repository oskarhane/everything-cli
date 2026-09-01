package account

import (
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/spf13/cobra"
)

// listAccount is one rendered row of account list. The default account is
// marked with default: true.
type listAccount struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Default bool   `json:"default"`
}

// newListCmd builds account list: every configured account, with the
// default account marked.
func newListCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured Google accounts",
		Example: `# List all configured accounts
everything-cli google account list

# List accounts as JSON; the default account carries "default": true
everything-cli google account list --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			accounts, err := store.List()
			if err != nil {
				return err
			}
			def, err := store.DefaultAccount()
			if err != nil {
				return err
			}

			rows := make([]listAccount, 0, len(accounts))
			for _, a := range accounts {
				rows = append(rows, listAccount{Name: a.Name, Email: a.Email, Default: a.Name == def})
			}

			tableRows := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				marker := ""
				if r.Default {
					marker = "(default)"
				}
				tableRows = append(tableRows, map[string]any{
					"name":    r.Name,
					"email":   r.Email,
					"default": marker,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "email", "default"}, rows, tableRows)
			return nil
		},
	}
}
