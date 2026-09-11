package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newChannelCmd is a placeholder for the `slack channel` resource tree; the
// slack-channel node replaces this file with the real parent wiring
// `channel history` and `channel list`. The stub is deliberately non-runnable
// (no Run/RunE and no children) so help and drift guards stay inert until
// then.
func newChannelCmd(_ *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "channel",
		Short: "Read Slack channels and their history",
	}
}
