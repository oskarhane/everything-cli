package thread

import (
	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/message"
)

// listFields is the field order for thread list output; messageFields is the
// per-message field order for `thread get`. The same names are the snake_case
// JSON and TOON keys. go-pretty's StyleLight upper-cases the headers when
// rendering.
var (
	listFields    = []string{"id", "snippet", "messages_count"}
	messageFields = []string{"id", "from", "subject", "date", "snippet"}
)

// threadRow maps one thread to its output row; messages_count is the number
// of messages the thread holds.
func threadRow(t *gmail.Thread) map[string]any {
	return map[string]any{
		"id":             t.Id,
		"snippet":        t.Snippet,
		"messages_count": len(t.Messages),
	}
}

// messageRow maps one message of a thread to its output row.
func messageRow(m *gmail.Message) map[string]any {
	return map[string]any{
		"id":      m.Id,
		"from":    message.HeaderValue(m, "From"),
		"subject": message.HeaderValue(m, "Subject"),
		"date":    message.HeaderValue(m, "Date"),
		"snippet": m.Snippet,
	}
}

// printThreads renders zero or more threads in the resolved output format.
func printThreads(cmd *cobra.Command, cfg *app.Config, threads []*gmail.Thread) {
	rows := make([]map[string]any, 0, len(threads))
	for _, t := range threads {
		rows = append(rows, threadRow(t))
	}
	renderRows(cmd, cfg, listFields, rows)
}

// printThreadMessages renders the messages of one thread.
func printThreadMessages(cmd *cobra.Command, cfg *app.Config, thread *gmail.Thread) {
	rows := make([]map[string]any, 0, len(thread.Messages))
	for _, m := range thread.Messages {
		rows = append(rows, messageRow(m))
	}
	renderRows(cmd, cfg, messageFields, rows)
}

// renderRows writes rows in the resolved output format.
func renderRows(cmd *cobra.Command, cfg *app.Config, fields []string, rows []map[string]any) {
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), fields, rows, rows)
}
