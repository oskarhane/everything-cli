package account

import (
	"strings"
	"time"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/spf13/cobra"
)

// getAccount is the rendered shape of account get: identity, granted scopes
// and token expiry. OAuth token values are deliberately absent from the
// view, so no output format can leak them.
type getAccount struct {
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Scopes      []string `json:"scopes"`
	TokenExpiry string   `json:"token_expiry"`
}

// newGetCmd builds account get: one account's metadata. Token values are
// never printed.
func newGetCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show a Google account's email, scopes and token expiry",
		Example: `# Show the "work" account
everything-cli account get work

# Show the "work" account as JSON (token values are never printed)
everything-cli account get work --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			a, err := store.Get(args[0])
			if err != nil {
				return err
			}

			view := getAccount{
				Name:   a.Name,
				Email:  a.Email,
				Scopes: a.Scopes,
			}
			if a.Token != nil && !a.Token.Expiry.IsZero() {
				view.TokenExpiry = a.Token.Expiry.UTC().Format(time.RFC3339)
			}

			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "email", "scopes", "token_expiry"}, view,
				[]map[string]any{{
					"name":         view.Name,
					"email":        view.Email,
					"scopes":       strings.Join(view.Scopes, ", "),
					"token_expiry": view.TokenExpiry,
				}})
			return nil
		},
	}
}
