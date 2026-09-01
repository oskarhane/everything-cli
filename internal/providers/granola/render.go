package granola

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// noteListFields is the note list field order for table output; the same
// snake_case names are the JSON and TOON keys. noteViewFields is the single
// note field order (get adds web_url). go-pretty's StyleLight upper-cases
// the headers when rendering.
var (
	noteListFields = []string{"note_id", "title", "owner", "created_at", "updated_at"}
	noteViewFields = []string{"note_id", "title", "owner", "created_at", "updated_at", "web_url"}
)

// noteRow maps one note summary to its output row. The API's id renders as
// note_id so output keys stay self-describing across providers; owner is
// the owner email (name may be null).
func noteRow(n NoteSummary) map[string]any {
	return map[string]any{
		"note_id":    n.ID,
		"title":      deref(n.Title),
		"owner":      n.Owner.Email,
		"created_at": n.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": n.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// printNoteList renders zero or more notes: a JSON/TOON array, or a table
// with one row per note, in the resolved output format.
func printNoteList(cmd *cobra.Command, cfg *app.Config, notes []NoteSummary) {
	rows := make([]map[string]any, 0, len(notes))
	for _, n := range notes {
		rows = append(rows, noteRow(n))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), noteListFields, rows, rows)
}

// printNoteView renders a single note: the full note (every field, snake_case
// keys) in JSON/TOON, a one-row summary table otherwise. The API's id key is
// renamed to note_id for output. Marshal/unmarshal of this struct cannot
// realistically fail, but data-driven marshal failures must return an error,
// never panic.
func printNoteView(cmd *cobra.Command, cfg *app.Config, n *Note) error {
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	var view map[string]any
	if err := json.Unmarshal(data, &view); err != nil {
		return err
	}
	view["note_id"] = view["id"]
	delete(view, "id")

	tableRow := map[string]any{
		"note_id":    n.ID,
		"title":      deref(n.Title),
		"owner":      n.Owner.Email,
		"created_at": n.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": n.UpdatedAt.UTC().Format(time.RFC3339),
		"web_url":    n.WebURL,
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), noteViewFields, view, []map[string]any{tableRow})
	return nil
}

// deref renders a nullable string field as its value or "".
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
