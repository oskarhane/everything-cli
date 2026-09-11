package slack

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// defaultChannelMax is the default --max shared by both channel listing
// leaves: about one Slack page of items, so the common case makes a single
// request while a busy workspace cannot page forever.
const defaultChannelMax = 25

// channelHistoryFields is the table column order; the same snake_case names
// are the JSON and TOON keys (AGENTS.md casing rule). go-pretty's StyleLight
// upper-cases the header cells at render time.
var channelHistoryFields = []string{"ts", "channel_id", "user", "text", "thread_ts", "reply_count", "reactions", "edited"}

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
	f.Int64Var(&opts.Max, "max", defaultChannelMax, "Total max messages across all pages (0 = no cap)")
	_ = cmd.MarkFlagRequired("channel")
	return cmd
}

// printChannelHistory renders messages in the resolved format: JSON/TOON as
// the shared Message shape, or one table row per message.
func printChannelHistory(cmd *cobra.Command, cfg *app.Config, messages []Message) {
	jsonRows := make([]map[string]any, 0, len(messages))
	tableRows := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		jsonRows = append(jsonRows, channelHistoryJSONRow(m))
		tableRows = append(tableRows, channelHistoryTableRow(m))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), channelHistoryFields, jsonRows, tableRows)
}

// channelHistoryJSONRow maps one message to its JSON/TOON row, mirroring the
// shared Message tags: thread_ts and reactions are omitted when empty,
// reply_count and edited always render.
func channelHistoryJSONRow(m Message) map[string]any {
	row := map[string]any{
		"ts":          m.TS,
		"channel_id":  m.ChannelID,
		"user":        m.User,
		"text":        m.Text,
		"reply_count": m.ReplyCount,
		"edited":      m.Edited,
	}
	if m.ThreadTS != "" {
		row["thread_ts"] = m.ThreadTS
	}
	if len(m.Reactions) > 0 {
		row["reactions"] = m.Reactions
	}
	return row
}

// channelHistoryTableRow flattens the same message into table cells: every
// column is present so rows stay aligned, and reactions join into one cell.
func channelHistoryTableRow(m Message) map[string]any {
	reactions := make([]string, 0, len(m.Reactions))
	for _, r := range m.Reactions {
		reactions = append(reactions, fmt.Sprintf("%s:%d", r.Name, r.Count))
	}
	return map[string]any{
		"ts":          m.TS,
		"channel_id":  m.ChannelID,
		"user":        m.User,
		"text":        m.Text,
		"thread_ts":   m.ThreadTS,
		"reply_count": m.ReplyCount,
		"reactions":   strings.Join(reactions, ","),
		"edited":      m.Edited,
	}
}
