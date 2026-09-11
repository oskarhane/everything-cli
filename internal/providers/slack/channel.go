package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

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
