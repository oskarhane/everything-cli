package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newSearchCmd builds the `slack search` resource tree: full-text message
// search (user tokens only).
func newSearchCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Slack messages",
	}
	cmd.AddCommand(newSearchMessagesCmd(cfg))
	return cmd
}
