package account

import (
	"fmt"
	"sort"
	"strings"

	provideraccount "github.com/oskarhane/everything-cli/internal/account"
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/spf13/cobra"
)

// listAccount is one rendered row of account list: which provider owns the
// account, how the provider identifies it, and whether it is its provider's
// default. Secrets (tokens, API keys) are deliberately absent — no output
// format may ever carry them.
type listAccount struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Identity string `json:"identity"`
	Default  bool   `json:"default"`
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
everything-cli account list

# List accounts as JSON; each provider's default carries "default": true
everything-cli account list --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := cfg.Store()
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
					Name:     a.Name,
					Provider: a.Provider,
					Identity: identity(a),
					Default:  a.Name == def,
				})
			}

			tableRows := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, map[string]any{
					"name":     r.Name,
					"provider": r.Provider,
					"identity": r.Identity,
					"default":  provideraccount.DefaultMarker(r.Default),
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "provider", "identity", "default"}, rows, tableRows)
			return nil
		},
	}
}
