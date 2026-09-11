package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// channelHistoryFields is the table column order; the same snake_case names
// are the JSON and TOON keys (AGENTS.md casing rule). go-pretty's StyleLight
// upper-cases the header cells at render time.
var channelHistoryFields = []string{"ts", "channel_id", "user", "text", "thread_ts", "reply_count", "reactions", "edited", "files"}

// newChannelHistoryCmd returns `slack channel history`: one conversation's
// messages, newest first, following cursor pagination up to --max.
func newChannelHistoryCmd(cfg *app.Config) *cobra.Command {
	var opts ChannelHistoryOptions
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Read messages in a Slack conversation, newest first",
		Example: `# The 25 most recent messages in a conversation as JSON
everything-cli slack channel history --channel C0B3HMXFEUV --format json

# Messages in a time window as a table
everything-cli slack channel history --channel C0B3HMXFEUV --oldest 1512085950.000216 --latest 1512090000.000000 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			messages, err := svc.ChannelHistory(cmd.Context(), opts)
			if err != nil {
				return err
			}
			printChannelHistory(cmd, cfg, messages)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Channel, "channel", "", "Conversation id (C..., D..., or G...); required")
	f.StringVar(&opts.Oldest, "oldest", "", "Only messages at or after this ts (e.g. 1512085950.000216)")
	f.StringVar(&opts.Latest, "latest", "", "Only messages at or before this ts")
	f.IntVar(&opts.Max, "max", defaultChannelMax, "Total max messages across all pages (0 = no cap)")
	_ = cmd.MarkFlagRequired("channel")
	return cmd
}

// printChannelHistory renders messages in the resolved format: one shared
// messageRow per message feeds both the JSON/TOON output and the table.
func printChannelHistory(cmd *cobra.Command, cfg *app.Config, messages []Message) {
	rows := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		rows = append(rows, messageRow(m))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), channelHistoryFields, rows, rows)
}
