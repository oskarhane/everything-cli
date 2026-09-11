package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newUserCmd is a placeholder for the `slack user` resource tree; the
// slack-user node replaces this file with the real parent wiring
// `user get` and `user list`. The stub is deliberately non-runnable (no
// Run/RunE and no children) so help and drift guards stay inert until then.
func newUserCmd(_ *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "user",
		Short: "Look up Slack workspace members",
	}
}
