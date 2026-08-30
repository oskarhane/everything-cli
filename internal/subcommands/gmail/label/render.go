package label

import (
	"io"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// labelFields is the label field order for table output; the same names are
// the snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases the
// headers when rendering.
var labelFields = []string{"id", "name", "type", "unread_count", "messages_total", "threads_total"}

// labelRow maps one Gmail label to its output row.
func labelRow(l *gmail.Label) map[string]any {
	return map[string]any{
		"id":             l.Id,
		"name":           l.Name,
		"type":           l.Type,
		"unread_count":   l.MessagesUnread,
		"messages_total": l.MessagesTotal,
		"threads_total":  l.ThreadsTotal,
	}
}

// printLabels renders zero or more labels: a JSON/TOON array, or a table with
// one row per label, in the resolved output format.
func printLabels(cmd *cobra.Command, cfg *app.Config, labels []*gmail.Label) {
	rows := make([]map[string]any, 0, len(labels))
	for _, l := range labels {
		rows = append(rows, labelRow(l))
	}
	render(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), rows, rows)
}

// printLabel renders a single label: an object in JSON/TOON, a one-row table.
func printLabel(cmd *cobra.Command, cfg *app.Config, l *gmail.Label) {
	row := labelRow(l)
	render(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), row, []map[string]any{row})
}

// render writes v (JSON/TOON) or rows (table) in the given format.
func render(w io.Writer, format output.Format, v any, rows []map[string]any) {
	switch format {
	case output.FormatTable:
		output.PrintTable(w, labelFields, rows)
	case output.FormatToon:
		output.PrintToon(w, v)
	default:
		output.PrintJSON(w, v)
	}
}
