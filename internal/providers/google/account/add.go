package account

import (
	"fmt"

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

			creds, err := resolveClientCredentials(cfg, store, credentials)
			if err != nil {
				return err
			}

			scopes := auth.ParseScopes(scopesFlag)
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

// resolveClientCredentials turns the --credentials flag value (falling back
// to the provider-level cfg.Credentials) into parsed OAuth app credentials:
// resolve the path against the store's config dir, then read and parse the
// file. Onboarding (add) and re-authorization (auth) need the identical
// chain, so it lives here once — add owns onboarding — instead of a copy
// per leaf.
func resolveClientCredentials(cfg *app.Config, store *config.Store, flagValue string) (auth.ClientCredentials, error) {
	credentialsPath := flagValue
	if credentialsPath == "" {
		credentialsPath = cfg.Credentials
	}
	resolved, err := auth.ResolveCredentials(cfg.Fs, credentialsPath, store.Dir())
	if err != nil {
		return auth.ClientCredentials{}, err
	}
	return auth.ReadClientCredentials(cfg.Fs, resolved)
}
