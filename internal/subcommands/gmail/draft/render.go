package draft

import (
	"strings"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// listFields is the field order for draft list output (and the post-create /
// post-send echo); detailFields is the single-draft field order. The same
// names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var (
	listFields   = []string{"id", "message_id", "snippet"}
	detailFields = []string{"id", "message_id", "from", "to", "subject", "date", "snippet"}
)

// draftRow maps one draft to its summary output row.
func draftRow(d *gmail.Draft) map[string]any {
	row := map[string]any{"id": d.Id}
	if d.Message != nil {
		row["message_id"] = d.Message.Id
		row["snippet"] = d.Message.Snippet
	}
	return row
}

// detailRow extends draftRow with the stored message's headers, the fields
// users read a single draft for.
func draftDetailRow(d *gmail.Draft) map[string]any {
	row := draftRow(d)
	if d.Message != nil {
		row["from"] = headerValue(d.Message, "From")
		row["to"] = headerValue(d.Message, "To")
		row["subject"] = headerValue(d.Message, "Subject")
		row["date"] = headerValue(d.Message, "Date")
	}
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

// printDrafts renders zero or more drafts in the resolved output format.
func printDrafts(cmd *cobra.Command, cfg *app.Config, drafts []*gmail.Draft) {
	rows := make([]map[string]any, 0, len(drafts))
	for _, d := range drafts {
		rows = append(rows, draftRow(d))
	}
	renderRows(cmd, cfg, listFields, rows)
}

// printDraft renders a single draft summary (the post-mutation echo).
func printDraft(cmd *cobra.Command, cfg *app.Config, d *gmail.Draft) {
	renderRows(cmd, cfg, listFields, []map[string]any{draftRow(d)})
}

// printDraftDetail renders a single draft with its stored message headers.
func printDraftDetail(cmd *cobra.Command, cfg *app.Config, d *gmail.Draft) {
	renderRows(cmd, cfg, detailFields, []map[string]any{draftDetailRow(d)})
}

// printSent renders the message a draft send produced.
func printSent(cmd *cobra.Command, cfg *app.Config, m *gmail.Message) {
	row := map[string]any{"id": m.Id, "thread_id": m.ThreadId, "snippet": m.Snippet}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
		[]string{"id", "thread_id", "snippet"}, row, []map[string]any{row})
}

// renderRows writes rows in the resolved output format.
func renderRows(cmd *cobra.Command, cfg *app.Config, fields []string, rows []map[string]any) {
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), fields, rows, rows)
}
