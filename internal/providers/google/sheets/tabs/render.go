package tabs

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// Field orders for the tabs leaves' mutation echoes; the same names are the
// snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases the
// headers when rendering.
var (
	createFields = []string{"sheet_id", "title"}
	deleteFields = []string{"deleted_tab"}
	renameFields = []string{"old_title", "new_title"}
)

// printCreated renders the echo after a create: the new tab's title and the
// sheet id the API assigned it.
func printCreated(cmd *cobra.Command, cfg *app.Config, title string, sheetID int64) {
	printOne(cmd, cfg, createFields, map[string]any{"sheet_id": sheetID, "title": title})
}

// printDeleted renders the echo after a delete.
func printDeleted(cmd *cobra.Command, cfg *app.Config, title string) {
	printOne(cmd, cfg, deleteFields, map[string]any{"deleted_tab": title})
}

// printRenamed renders the echo after a rename: the tab's old and new title.
func printRenamed(cmd *cobra.Command, cfg *app.Config, oldTitle, newTitle string) {
	printOne(cmd, cfg, renameFields, map[string]any{"old_title": oldTitle, "new_title": newTitle})
}

// printOne renders a single output object (a mutation echo): an object in
// JSON/TOON, a one-row table with the same fields.
func printOne(cmd *cobra.Command, cfg *app.Config, fields []string, row map[string]any) {
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), fields, row, []map[string]any{row})
}
