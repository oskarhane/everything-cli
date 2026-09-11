package account

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// identityGetView is the rendered shape of account get for providers whose
// accounts carry an email identity and OAuth token metadata (google):
// identity, granted scopes and token expiry. Token values are deliberately
// absent from the view, so no output format can leak them.
type identityGetView struct {
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Scopes      []string `json:"scopes"`
	TokenExpiry string   `json:"token_expiry"`
}

// NewGetCmd builds account get for the provider described by spec: one
// account's metadata. Secrets (tokens, API keys) are never printed.
func NewGetCmd(cfg *app.Config, spec Spec) *cobra.Command {
	short := "Show a " + spec.DisplayName + " account's metadata"
	if spec.Identity {
		short = "Show a " + spec.DisplayName + " account's email, scopes and token expiry"
	}
	return &cobra.Command{
		Use:   "get <name>",
		Short: short,
		Example: `# Show the "work" account
everything-cli ` + spec.ProviderID + ` account get work

# Show the "work" account as JSON (secrets are never printed)
everything-cli ` + spec.ProviderID + ` account get work --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}
			a, err := store.GetProvider(spec.ProviderID, args[0])
			if err != nil {
				return err
			}

			format := output.ResolveOutput(cfg.Format)
			if spec.Identity {
				view := identityGetView{
					Name:   a.Name,
					Email:  a.Email,
					Scopes: a.Scopes,
				}
				if a.Token != nil && !a.Token.Expiry.IsZero() {
					view.TokenExpiry = a.Token.Expiry.UTC().Format(time.RFC3339)
				}
				output.Print(cmd.OutOrStdout(), format,
					[]string{"name", "email", "scopes", "token_expiry"}, view,
					[]map[string]any{{
						"name":         view.Name,
						"email":        view.Email,
						"scopes":       strings.Join(view.Scopes, ", "),
						"token_expiry": view.TokenExpiry,
					}})
				return nil
			}

			// Key-based providers: name and provider, plus any stored
			// Identity entries (slack's auth.test metadata) as additional
			// top-level fields in deterministic key order. Keys that would
			// shadow name/provider are skipped. The credential is absent
			// from the row, so no output format can leak it.
			fields := []string{"name", "provider"}
			row := map[string]any{"name": a.Name, "provider": a.Provider}
			identityKeys := make([]string, 0, len(a.Identity))
			for k := range a.Identity {
				if k == "name" || k == "provider" {
					continue
				}
				identityKeys = append(identityKeys, k)
			}
			sort.Strings(identityKeys)
			for _, k := range identityKeys {
				fields = append(fields, k)
				row[k] = a.Identity[k]
			}
			output.Print(cmd.OutOrStdout(), format, fields, row, []map[string]any{row})
			return nil
		},
	}
}
