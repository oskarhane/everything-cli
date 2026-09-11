package slack

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/output"
)

// addedAccount is the rendered shape of a successful account add: the name
// only. The token is never part of any output.
type addedAccount struct {
	Name string `json:"name"`
}

// newAccountAddCmd builds `slack account add`: capture an xoxp user token
// (--api-key flag, then SLACK_API_KEY, then a hidden prompt — never echoed),
// validate it with auth.test, and persist it as a provider-scoped account.
// The strategy registers the token for redaction at capture, before anything
// could print it.
func newAccountAddCmd(cfg *app.Config) *cobra.Command {
	var apiKey string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a Slack account from an xoxp user token",
		Example: `# Add the "work" account, entering the token at a hidden prompt
everything-cli slack account add work

# Add it non-interactively from the environment
SLACK_API_KEY=xoxp-... everything-cli slack account add work

# Or pass the token directly (careful: shell history)
everything-cli slack account add work --api-key xoxp-...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}
			acct, err := strategy.Add(cmd.Context(), cfg.Fs, store, auth.AddOptions{
				Name:   args[0],
				APIKey: apiKey,
			})
			if err != nil {
				return fmt.Errorf("adding slack account %q: %w", args[0], err)
			}
			view := addedAccount{Name: acct.Name}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name"}, view,
				[]map[string]any{{"name": view.Name}})
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "",
		"Slack user token (xoxp-...; empty = SLACK_API_KEY env var, then a hidden prompt)")
	return cmd
}
