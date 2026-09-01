package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// addedAccount is the rendered shape of a successful account add. The API
// key is deliberately absent from the view, so no output format can leak
// it.
type addedAccount struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// newAddCmd builds account add: capture a Linear personal API key — from
// --api-key, then the provider's env var, then a hidden prompt — and store
// it as a provider account. The key is never printed; the strategy
// registers it for redaction at capture.
func newAddCmd(cfg *app.Config, providerID string, newStrategy StrategyFactory) *cobra.Command {
	var apiKey string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a Linear account with a personal API key",
		Example: `# Add the account "work", entering the key at a hidden prompt
everything-cli linear account add work

# Add "work" with the key from the environment
LINEAR_API_KEY=lin_api_... everything-cli linear account add work

# Add "work" passing the key explicitly (prefer the env var in scripts)
everything-cli linear account add work --api-key lin_api_...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			acct, err := newStrategy().Add(cmd.Context(), cfg.Fs, store, auth.AddOptions{
				Name:   args[0],
				APIKey: apiKey,
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
	return cmd
}
