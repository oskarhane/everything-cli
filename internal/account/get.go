package account

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
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

// getView is the rendered shape of account get for key-based providers:
// account metadata only. The credential is deliberately absent from the
// view, so no output format can leak it.
type getView struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
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
			store, err := config.NewStore(cfg.Fs, "")
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

			view := getView{Name: a.Name, Provider: a.Provider}
			output.Print(cmd.OutOrStdout(), format,
				[]string{"name", "provider"}, view,
				[]map[string]any{{"name": view.Name, "provider": view.Provider}})
			return nil
		},
	}
}
