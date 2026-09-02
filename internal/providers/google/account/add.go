package account

import (
	"fmt"
	"strings"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/spf13/cobra"
)

// newAddStrategy is the auth-strategy seam: the OAuth flow runs through the
// provider's auth.Strategy, never directly. Production wires the pinned
// Google OAuth profile's strategy (the same construction the google
// provider package's NewStrategy wraps — imported directly here because the
// account tree living inside the google provider may not import its parent);
// tests stub it so no test ever starts a real browser authorization.
var newAddStrategy = func(store *config.Store, creds auth.ClientCredentials) auth.Strategy {
	return auth.NewOAuthStrategy(auth.GoogleOAuth, store, creds)
}

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
everything-cli google account add work

# Authorize "work" with an explicit credentials file and scope set
everything-cli google account add work --credentials ~/google/credentials.json --scopes https://www.googleapis.com/auth/gmail.send`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
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
			creds, err := auth.ReadClientCredentials(cfg.Fs, resolved)
			if err != nil {
				return err
			}

			scopes := parseScopes(scopesFlag)
			strategy := newAddStrategy(store, creds)
			acct, err := strategy.Add(cmd.Context(), cfg.Fs, store, auth.AddOptions{
				Name:        args[0],
				Credentials: creds,
				Scopes:      scopes,
			})
			if err != nil {
				return fmt.Errorf("adding account %q: %w", args[0], err)
			}

			view := addedAccount{Name: acct.Name, Email: acct.Email}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "email"}, view,
				[]map[string]any{{"name": view.Name, "email": view.Email}})
			return nil
		},
	}
	cmd.Flags().StringVar(&credentials, "credentials", "",
		"Path to OAuth app credentials JSON (empty = auto-resolve)")
	cmd.Flags().StringVar(&scopesFlag, "scopes", "",
		"Comma-separated OAuth scopes (empty = Gmail + Calendar + Drive + Docs + Sheets + Slides + userinfo.email)")
	return cmd
}

// parseScopes splits a comma-separated --scopes value, trimming blanks. An
// empty value yields nil, and the strategy falls back to the profile's
// default scope set (auth.GoogleOAuth.DefaultScopes).
func parseScopes(flagValue string) []string {
	if flagValue == "" {
		return nil
	}
	scopes := make([]string, 0, 4)
	for _, s := range strings.Split(flagValue, ",") {
		if s = strings.TrimSpace(s); s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}
