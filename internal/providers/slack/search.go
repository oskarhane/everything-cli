package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newSearchCmd is a placeholder for the `slack search` resource tree; the
// slack-search-messages node replaces this file with the real parent wiring
// `search messages`. The stub is deliberately non-runnable (no Run/RunE and
// no children) so help and drift guards stay inert until then.
func newSearchCmd(_ *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "search",
		Short: "Search Slack messages",
	}
}
