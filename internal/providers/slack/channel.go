package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// defaultChannelMax is the default --max shared by both channel listing
// leaves: about one Slack page of items, so the common case makes a single
// request while a busy workspace cannot page forever.
const defaultChannelMax = 25

// newChannelCmd returns the `slack channel` parent with every channel leaf
// attached, one AddCommand line each: history reads one conversation's
// messages, list enumerates the conversations the token can see.
func newChannelCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "Read Slack channels and their history",
	}
	cmd.AddCommand(newChannelHistoryCmd(cfg))
	cmd.AddCommand(newChannelListCmd(cfg))
	return cmd
}
