package account

import (
	"fmt"
	"strings"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/spf13/cobra"
)

// runFlow is the OAuth flow seam. Production wires auth.RunFlow; tests stub
// it so no test ever starts a real browser authorization.
var runFlow = auth.RunFlow

// addedAccount is the rendered shape of a successful account add.
type addedAccount struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// newAddCmd builds account add: authorize a Google account with the OAuth
// flow and cache its token.
func newAddCmd(cfg *app.Config) *cobra.Command {
	var credentials, scopesFlag string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Authorize a Google account via the OAuth flow",
		Example: `# Authorize a new account named "work" with the default scopes
google-cli account add work

# Authorize "work" with an explicit credentials file and scope set
google-cli account add work --credentials ~/google/credentials.json --scopes https://www.googleapis.com/auth/gmail.send`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}

			credentialsPath := credentials
			if credentialsPath == "" {
				credentialsPath = cfg.Credentials
			}
			resolved, err := auth.ResolveCredentials(cfg.Fs, credentialsPath, store.Dir())
			if err != nil {
				return err
			}

			scopes := parseScopes(scopesFlag)
			tok, email, err := runFlow(resolved, scopes)
			if err != nil {
				return fmt.Errorf("authorizing account %q: %w", args[0], err)
			}

			saved, err := auth.SaveAccount(store, args[0], email, scopes, tok)
			if err != nil {
				return err
			}

			view := addedAccount{Name: saved, Email: email}
			w := cmd.OutOrStdout()
			switch output.ResolveOutput(cfg.Format) {
			case output.FormatTable:
				output.PrintTable(w, []string{"name", "email"},
					[]map[string]any{{"name": view.Name, "email": view.Email}})
			case output.FormatToon:
				output.PrintToon(w, view)
			default:
				output.PrintJSON(w, view)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&credentials, "credentials", "",
		"Path to OAuth app credentials JSON (empty = auto-resolve)")
	cmd.Flags().StringVar(&scopesFlag, "scopes", "",
		"Comma-separated OAuth scopes (empty = Gmail + Calendar + userinfo.email)")
	return cmd
}

// defaultScopes returns the scopes granted when --scopes is not given: full
// Gmail and Calendar access, plus userinfo.email.
func defaultScopes() []string {
	scopes := append([]string{}, auth.ScopesGmail...)
	scopes = append(scopes, auth.ScopesCalendar...)
	return append(scopes, auth.ScopeUserEmail)
}

// parseScopes splits a comma-separated --scopes value, trimming blanks. An
// empty value yields the default scope set.
func parseScopes(flagValue string) []string {
	if flagValue == "" {
		return defaultScopes()
	}
	scopes := make([]string, 0, 4)
	for _, s := range strings.Split(flagValue, ",") {
		if s = strings.TrimSpace(s); s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}
