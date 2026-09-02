package account

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// listRow is one rendered row of account list. The provider's default
// account is marked with default: true.
type listRow struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

// identityListRow is the list row for providers whose accounts carry an
// email identity (google).
type identityListRow struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Default bool   `json:"default"`
}

// NewListCmd builds account list for the provider described by spec: every
// configured account of that provider, with the default account marked.
// Secrets (tokens, API keys) are never part of the view.
func NewListCmd(cfg *app.Config, spec Spec) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured " + spec.DisplayName + " accounts",
		Example: `# List all configured ` + spec.DisplayName + ` accounts
everything-cli ` + spec.ProviderID + ` account list

# List accounts as JSON; the default account carries "default": true
everything-cli ` + spec.ProviderID + ` account list --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}
			accounts, err := store.ListProvider(spec.ProviderID)
			if err != nil {
				return err
			}
			def, err := store.DefaultAccountFor(spec.ProviderID)
			if err != nil {
				return err
			}

			format := output.ResolveOutput(cfg.Format)
			if spec.Identity {
				rows := make([]identityListRow, 0, len(accounts))
				tableRows := make([]map[string]any, 0, len(accounts))
				for _, a := range accounts {
					isDefault := a.Name == def
					rows = append(rows, identityListRow{Name: a.Name, Email: a.Email, Default: isDefault})
					tableRows = append(tableRows, map[string]any{
						"name":    a.Name,
						"email":   a.Email,
						"default": DefaultMarker(isDefault),
					})
				}
				output.Print(cmd.OutOrStdout(), format,
					[]string{"name", "email", "default"}, rows, tableRows)
				return nil
			}

			rows := make([]listRow, 0, len(accounts))
			tableRows := make([]map[string]any, 0, len(accounts))
			for _, a := range accounts {
				isDefault := a.Name == def
				rows = append(rows, listRow{Name: a.Name, Default: isDefault})
				tableRows = append(tableRows, map[string]any{
					"name":    a.Name,
					"default": DefaultMarker(isDefault),
				})
			}
			output.Print(cmd.OutOrStdout(), format,
				[]string{"name", "default"}, rows, tableRows)
			return nil
		},
	}
}

// DefaultMarker renders the table's default column: a marker on the
// default account's row, empty elsewhere. Shared by the per-provider and
// cross-provider account lists so both mark defaults identically.
func DefaultMarker(isDefault bool) string {
	if isDefault {
		return "(default)"
	}
	return ""
}
