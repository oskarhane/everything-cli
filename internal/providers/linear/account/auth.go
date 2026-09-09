package account

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/output"
)

// newAuthCmd builds account auth: re-run the browser OAuth flow for an
// existing OAuth account and replace its stored token in place, under the
// same name. The account's identity is pinned — authorizing as a different
// Linear user fails instead of corrupting the account — and the OAuth app
// credentials are re-read from the stored account, so there is no
// --client-id/--client-secret to pass (and no credentials file, unlike
// google). API-key accounts are rejected: they have no flow to re-run.
func newAuthCmd(cfg *app.Config, providerID string, newStrategy StrategyFactory) *cobra.Command {
	var scopesFlag string
	cmd := &cobra.Command{
		Use:   "auth <name>",
		Short: "Re-authorize an existing Linear OAuth account via the OAuth flow",
		Example: `# Re-authorize "work" after revoking the CLI's OAuth app in Linear settings
everything-cli linear account auth work

# Re-consent "work" with a narrowed scope set
everything-cli linear account auth work --scopes read`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}
			acct, err := store.GetProvider(providerID, args[0])
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("no linear account %q; run \"everything-cli linear account add %s\" first", args[0], args[0])
				}
				return err
			}

			// Belt-only: the composite strategy always implements
			// Reauther, but the seam is auth.Strategy, so the capability
			// is type-asserted rather than assumed.
			reauther, ok := newStrategy(store).(auth.Reauther)
			if !ok {
				return errors.New("linear accounts do not support re-auth")
			}
			acct, err = reauther.Reauth(cmd.Context(), cfg.Fs, store, acct, auth.ReauthOptions{
				Scopes: parseScopes(scopesFlag),
			})
			if err != nil {
				return fmt.Errorf("re-authorizing account %q: %w", args[0], err)
			}

			view := addedAccount{Name: acct.Name, Provider: acct.Provider}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name", "provider"}, view,
				[]map[string]any{{"name": view.Name, "provider": view.Provider}})
			return nil
		},
	}
	cmd.Flags().StringVar(&scopesFlag, "scopes", "",
		"Comma-separated OAuth scopes (empty = the account's current scopes)")
	return cmd
}

// parseScopes splits a comma-separated --scopes value, trimming blanks. An
// empty value yields nil, and re-auth keeps the account's currently granted
// scopes — a deliberately narrowed grant survives re-authorization.
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
