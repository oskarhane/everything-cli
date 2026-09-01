package account

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/spf13/cobra"
)

// listAccount is one rendered row of account list: which provider owns the
// account, how the provider identifies it, and whether it is its provider's
// default. Secrets (tokens, API keys) are deliberately absent — no output
// format may ever carry them.
type listAccount struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Identity  string `json:"identity"`
	IsDefault bool   `json:"is_default"`
}

// identity renders the account's human identity: the Google email when set,
// else the generic identity map as sorted key=value pairs.
func identity(a config.Account) string {
	if a.Email != "" {
		return a.Email
	}
	pairs := make([]string, 0, len(a.Identity))
	keys := make([]string, 0, len(a.Identity))
	for k := range a.Identity {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, a.Identity[k]))
	}
	return strings.Join(pairs, ", ")
}

// newListCmd builds account list: every account of every provider, with
// each provider's default marked.
func newListCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts across all providers",
		Example: `# List every configured account, across all providers
google-cli account list

# List accounts as JSON; each provider's default carries "is_default": true
google-cli account list --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			accounts, err := store.ListAll()
			if err != nil {
				return err
			}

			// Defaults are per provider; cache the lookup so N accounts
			// cost one settings read per provider, not per account.
			defaults := map[string]string{}
			rows := make([]listAccount, 0, len(accounts))
			for _, a := range accounts {
				def, ok := defaults[a.Provider]
				if !ok {
					def, err = store.DefaultAccountFor(a.Provider)
					if err != nil {
						return err
					}
					defaults[a.Provider] = def
				}
				rows = append(rows, listAccount{
					Name:      a.Name,
					Provider:  a.Provider,
					Identity:  identity(a),
					IsDefault: a.Name == def,
				})
			}

			tableRows := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				marker := ""
				if r.IsDefault {
					marker = "yes"
				}
				tableRows = append(tableRows, map[string]any{
					"name":       r.Name,
					"provider":   r.Provider,
					"identity":   r.Identity,
					"is_default": marker,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "provider", "identity", "is_default"}, rows, tableRows)
			return nil
		},
	}
}
