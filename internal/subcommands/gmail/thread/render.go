package thread

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
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
		"from":    headerValue(m, "From"),
		"subject": headerValue(m, "Subject"),
		"date":    headerValue(m, "Date"),
		"snippet": m.Snippet,
	}
}

// headerValue returns the named payload header (case-insensitive), or "" when
// the message carries no such header.
func headerValue(m *gmail.Message, name string) string {
	if m.Payload == nil {
		return ""
	}
	for _, h := range m.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
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
	w := cmd.OutOrStdout()
	switch output.ResolveOutput(cfg.Format) {
	case output.FormatTable:
		output.PrintTable(w, fields, rows)
	case output.FormatToon:
		printToon(w, rows)
	default:
		printJSON(w, rows)
	}
}

// printJSON writes one row as an object, several as an array.
func printJSON(w io.Writer, rows []map[string]any) {
	if len(rows) == 1 {
		output.PrintJSON(w, rows[0])
		return
	}
	output.PrintJSON(w, rows)
}

// printToon writes one row as an object, several as an array.
func printToon(w io.Writer, rows []map[string]any) {
	if len(rows) == 1 {
		output.PrintToon(w, rows[0])
		return
	}
	output.PrintToon(w, rows)
}
