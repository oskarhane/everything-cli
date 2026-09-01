package granola

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// addedAccount is the rendered shape of a successful account add: the name
// only. The API key is never part of any output.
type addedAccount struct {
	Name string `json:"name"`
}

// newAccountAddCmd builds `granola account add`: capture a grn_ API key
// (--api-key flag, then GRANOLA_API_KEY, then a hidden prompt — never
// echoed) and persist it as a provider-scoped account. The strategy
// registers the key for redaction at capture, before anything could print
// it.
func newAccountAddCmd(cfg *app.Config) *cobra.Command {
	var apiKey string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a Granola account from a grn_ API key",
		Example: `# Add the "work" account, entering the key at a hidden prompt
everything-cli granola account add work

# Add it non-interactively from the environment
GRANOLA_API_KEY=grn_... everything-cli granola account add work

# Or pass the key directly (careful: shell history)
everything-cli granola account add work --api-key grn_...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			acct, err := strategy.Add(cmd.Context(), cfg.Fs, store, auth.AddOptions{
				Name:   args[0],
				APIKey: apiKey,
			})
			if err != nil {
				return fmt.Errorf("adding granola account %q: %w", args[0], err)
			}
			view := addedAccount{Name: acct.Name}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name"}, view,
				[]map[string]any{{"name": view.Name}})
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "",
		"Granola API key (grn_...; empty = GRANOLA_API_KEY env var, then a hidden prompt)")
	return cmd
}
