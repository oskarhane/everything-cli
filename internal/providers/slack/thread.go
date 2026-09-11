package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// threadFields is the table field order for thread output; the same
// snake_case names are the JSON and TOON keys of the shared Message view.
// go-pretty's StyleLight upper-cases the headers when rendering.
var threadFields = []string{"ts", "user", "text", "thread_ts", "reply_count", "edited"}

// printThread renders a thread in the resolved output format: one shared
// messageRow per message feeds both the JSON/TOON output and the table
// (channel_id stays out of the table because every row shares it). Output's
// one-row convention collapses a one-message thread to a single JSON object /
// TOON document; a longer thread stays an array.
func printThread(cmd *cobra.Command, cfg *app.Config, messages []Message) {
	rows := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		rows = append(rows, messageRow(m))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), threadFields, rows, rows)
}

// newThreadCmd returns `slack thread`: one conversation thread — the parent
// message followed by its replies in Slack order. --channel and --ts select
// the thread and are required; --max budgets the messages across pages
// (0 = no cap).
func newThreadCmd(cfg *app.Config) *cobra.Command {
	var (
		channel  string
		ts       string
		maxItems int
	)
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Read a Slack thread: the parent message and its replies",
		Example: `# Show a thread as JSON
everything-cli slack thread --channel C0B3HMXFEUV --ts 1726038000.000100 --format json

# Show the parent plus at most 10 replies as a table
everything-cli slack thread --channel C0B3HMXFEUV --ts 1726038000.000100 --max 11 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			messages, err := svc.ThreadReplies(cmd.Context(), channel, ts, maxItems)
			if err != nil {
				return err
			}
			printThread(cmd, cfg, messages)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&channel, "channel", "", "Channel ID holding the thread, e.g. C0B3HMXFEUV (required)")
	f.StringVar(&ts, "ts", "", "Timestamp of the thread's parent message, e.g. 1726038000.000100 (required)")
	f.IntVar(&maxItems, "max", 25, "Maximum messages to return across pages (0 = no cap)")
	_ = cmd.MarkFlagRequired("channel")
	_ = cmd.MarkFlagRequired("ts")
	return cmd
}
