package account

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/spf13/cobra"
)

// newAuthStrategy is the re-authorization seam: like newAddStrategy it runs
// through the provider's auth strategy, never directly, and production wires
// the pinned Google OAuth profile's strategy — which carries the Reauth
// capability (identity pinned to the stored account). The seam is typed to
// auth.Reauther, not the broader auth.Strategy, because this leaf needs only
// re-authorization; tests stub it so no test ever starts a real browser
// authorization.
var newAuthStrategy = func(store *config.Store, creds auth.ClientCredentials) auth.Reauther {
	return auth.NewOAuthStrategy(auth.GoogleOAuth, store, creds)
}

// newAuthCmd builds account auth: re-run the OAuth flow for an existing
// account and replace its cached token in place.
func newAuthCmd(cfg *app.Config) *cobra.Command {
	var credentials, scopesFlag string
	cmd := &cobra.Command{
		Use:   "auth <name>",
		Short: "Re-authorize an existing Google account via the OAuth flow",
		Example: `# Re-authorize "work" after its refresh token was revoked
everything-cli google account auth work

# Re-consent "work" with a widened scope set
everything-cli google account auth work --scopes https://www.googleapis.com/auth/youtube.readonly`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}

			acct, err := store.GetProvider(config.ProviderGoogle, args[0])
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("no google account %q; run \"everything-cli google account add %s\" first", args[0], args[0])
				}
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

			strategy := newAuthStrategy(store, creds)
			acct, err = strategy.Reauth(cmd.Context(), cfg.Fs, store, acct, auth.ReauthOptions{
				Scopes: parseScopes(scopesFlag),
			})
			if err != nil {
				return fmt.Errorf("re-authorizing account %q: %w", args[0], err)
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
		"Comma-separated OAuth scopes (empty = keep the account's current scopes)")
	return cmd
}
