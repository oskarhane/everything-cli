package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// searchMessageFields is the search.messages table field order; the same
// snake_case names are the JSON/TOON keys where the shapes overlap.
// go-pretty's StyleLight upper-cases the headers when rendering.
var searchMessageFields = []string{"channel_id", "channel_name", "user", "username", "ts", "text", "permalink", "thread_ts", "files"}

// newSearchMessagesCmd returns `slack search messages`: full-text search
// across the workspace's messages (user token only), following Slack's page
// counter up to --max.
func newSearchMessagesCmd(cfg *app.Config) *cobra.Command {
	var (
		query string
		sort  string
		max   int
	)
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Search Slack messages",
		Example: `# The 25 most recent matches, as a table
everything-cli slack search messages --query "from:me deploy" --format table

# Newest matches first, capped at 3
everything-cli slack search messages --query "incident" --sort timestamp --max 3 --format json

# Every match across all pages (no cap)
everything-cli slack search messages --query "from:me" --max 0 --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			result, err := svc.SearchMessages(cmd.Context(), query, sort, max)
			if err != nil {
				return err
			}
			printSearchMessages(cmd, cfg, result)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&query, "query", "", "Search query (Slack search syntax, e.g. from:me or in:#general)")
	f.StringVar(&sort, "sort", "", "Sort order: score (Slack default) or timestamp")
	f.IntVar(&max, "max", 25, "Total max matches across all pages (0 = no cap)")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

// searchMessageRow maps one match to its table row.
func searchMessageRow(m SearchMatch) map[string]any {
	row := map[string]any{
		"channel_id":   m.ChannelID,
		"channel_name": m.ChannelName,
		"user":         m.User,
		"username":     m.Username,
		"ts":           m.TS,
		"text":         m.Text,
		"permalink":    m.Permalink,
		"thread_ts":    m.ThreadTS,
	}
	if len(m.Files) > 0 {
		row["files"] = fileCell(m.Files)
	}
	return row
}

// printSearchMessages renders the search view: the echoed query plus the
// matches as one JSON/TOON object, or a table with one row per match.
func printSearchMessages(cmd *cobra.Command, cfg *app.Config, result *SearchResult) {
	rows := make([]map[string]any, 0, len(result.Messages))
	for _, m := range result.Messages {
		rows = append(rows, searchMessageRow(m))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), searchMessageFields, result, rows)
}
