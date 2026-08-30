package message

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// listFields is the field order for message list output (and the summary echo
// after a mutation); detailFields is the single-message field order. The same
// names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var (
	listFields   = []string{"id", "thread_id", "snippet", "label_ids"}
	detailFields = []string{"id", "thread_id", "snippet", "label_ids", "from", "subject", "date"}
)

// messageRow maps one message to its output row.
func messageRow(m *gmail.Message) map[string]any {
	return map[string]any{
		"id":        m.Id,
		"thread_id": m.ThreadId,
		"snippet":   m.Snippet,
		"label_ids": m.LabelIds,
	}
}

// detailRow extends messageRow with the From, Subject, and Date payload
// headers, the fields users read a single message for.
func detailRow(m *gmail.Message) map[string]any {
	row := messageRow(m)
	row["from"] = headerValue(m, "From")
	row["subject"] = headerValue(m, "Subject")
	row["date"] = headerValue(m, "Date")
	return row
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

// printMessages renders zero or more messages: a JSON/TOON array, or a table
// with one row per message, in the resolved output format.
func printMessages(cmd *cobra.Command, cfg *app.Config, messages []*gmail.Message) {
	rows := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		rows = append(rows, messageRow(m))
	}
	render(cmd.OutOrStdout(), listFields, output.ResolveOutput(cfg.Format), rows...)
}

// printMessage renders a single message summary (the post-mutation echo).
func printMessage(cmd *cobra.Command, cfg *app.Config, m *gmail.Message) {
	render(cmd.OutOrStdout(), listFields, output.ResolveOutput(cfg.Format), messageRow(m))
}

// printMessageDetail renders a single message with its parsed headers.
func printMessageDetail(cmd *cobra.Command, cfg *app.Config, m *gmail.Message) {
	render(cmd.OutOrStdout(), detailFields, output.ResolveOutput(cfg.Format), detailRow(m))
}

// render writes rows in the given format. Tables compact label_ids into a
// comma-joined string so each cell stays on one line; JSON and TOON keep the
// array.
func render(w io.Writer, fields []string, format output.Format, rows ...map[string]any) {
	switch format {
	case output.FormatTable:
		tableRows := make([]map[string]any, len(rows))
		for i, row := range rows {
			tableRows[i] = compactRow(row)
		}
		output.PrintTable(w, fields, tableRows)
	case output.FormatToon:
		printToon(w, rows)
	default:
		printJSON(w, rows)
	}
}

// compactRow copies row with label_ids joined to a single string.
func compactRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		if k == "label_ids" {
			if ids, ok := v.([]string); ok {
				v = strings.Join(ids, ",")
			}
		}
		out[k] = v
	}
	return out
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
