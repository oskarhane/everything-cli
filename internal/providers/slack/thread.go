package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newThreadCmd is a placeholder for the runnable `slack thread` leaf; the
// slack-thread node replaces this file with the real leaf under the same
// constructor name. The stub is deliberately non-runnable (no Run/RunE and
// no children) so help and drift guards stay inert until then.
func newThreadCmd(_ *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "thread",
		Short: "Read a Slack thread and its replies",
	}
}
