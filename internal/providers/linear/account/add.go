package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/output"
)

// addedAccount is the rendered shape of a successful account add. The API
// key is deliberately absent from the view, so no output format can leak
// it.
type addedAccount struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// newAddCmd builds account add. The default path captures a Linear
// personal API key — from --api-key, then the provider's env var, then a
// hidden prompt; --oauth instead runs the browser OAuth flow (auth-code +
// PKCE, loopback redirect) with the app credentials from --client-id /
// LINEAR_CLIENT_ID and the PKCE-optional --client-secret /
// LINEAR_CLIENT_SECRET. Keys, secrets and tokens are never printed; the
// strategy registers them for redaction at capture.
func newAddCmd(cfg *app.Config, providerID string, newStrategy StrategyFactory) *cobra.Command {
	var apiKey, clientID, clientSecret string
	var useOAuth bool
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a Linear account with a personal API key or OAuth",
		Example: `# Add the account "work", entering the key at a hidden prompt
everything-cli linear account add work

# Add "work" with the key from the environment
LINEAR_API_KEY=lin_api_... everything-cli linear account add work

# Add "work" passing the key explicitly (prefer the env var in scripts)
everything-cli linear account add work --api-key lin_api_...

# Add "work" with OAuth (browser flow with PKCE); the client ID comes
# from the flag or LINEAR_CLIENT_ID, the secret is optional under PKCE
everything-cli linear account add work --oauth --client-id 7231...`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			// The OAuth app credential flags are meaningless on the
			// API-key path; fail fast instead of silently ignoring them.
			if useOAuth {
				return nil
			}
			for _, name := range []string{"client-id", "client-secret"} {
				if cmd.Flags().Changed(name) {
					return fmt.Errorf("--%s only applies with --oauth", name)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}
			acct, err := newStrategy(store).Add(cmd.Context(), cfg.Fs, store, auth.AddOptions{
				Name:         args[0],
				APIKey:       apiKey,
				UseOAuth:     useOAuth,
				ClientID:     clientID,
				ClientSecret: clientSecret,
			})
			if err != nil {
				return fmt.Errorf("adding account %q: %w", args[0], err)
			}

			view := addedAccount{Name: acct.Name, Provider: acct.Provider}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "provider"}, view,
				[]map[string]any{{"name": view.Name, "provider": view.Provider}})
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "",
		"Linear personal API key (empty = $LINEAR_API_KEY, then a hidden prompt)")
	cmd.Flags().BoolVar(&useOAuth, "oauth", false,
		"onboard with OAuth (browser flow with PKCE) instead of an API key")
	cmd.Flags().StringVar(&clientID, "client-id", "",
		"Linear OAuth app client ID (empty = $LINEAR_CLIENT_ID); --oauth only")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "",
		"Linear OAuth app client secret (optional under PKCE; empty = $LINEAR_CLIENT_SECRET); --oauth only")
	cmd.MarkFlagsMutuallyExclusive("api-key", "oauth")
	return cmd
}
