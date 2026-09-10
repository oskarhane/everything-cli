package tabs

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// Field orders for the tabs leaves' output; the same names are the snake_case
// JSON and TOON keys. go-pretty's StyleLight upper-cases the headers when
// rendering.
var (
	listFields   = []string{"tab_id", "title", "index", "nesting_level", "parent_tab_id"}
	createFields = []string{"tab_id", "title"}
	deleteFields = []string{"deleted_tab", "title"}
	renameFields = []string{"old_title", "new_title"}
)

// tabRow maps one document tab to its output row.
func tabRow(t service.DocTab) map[string]any {
	return map[string]any{
		"tab_id":        t.TabID,
		"title":         t.Title,
		"index":         t.Index,
		"nesting_level": t.NestingLevel,
		"parent_tab_id": t.ParentTabID,
	}
}

// printTabList renders zero or more document tabs: a JSON/TOON array, or a
// table with one row per tab, in the resolved output format.
func printTabList(cmd *cobra.Command, cfg *app.Config, docTabs []service.DocTab) {
	rows := make([]map[string]any, 0, len(docTabs))
	for _, t := range docTabs {
		rows = append(rows, tabRow(t))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), listFields, rows, rows)
}

// printCreated renders the echo after a create: the new tab's title and the
// tab id the API assigned it.
func printCreated(cmd *cobra.Command, cfg *app.Config, title, tabID string) {
	printOne(cmd, cfg, createFields, map[string]any{"tab_id": tabID, "title": title})
}

// printDeleted renders the echo after a delete: the deleted tab's resolved
// ID and its title, so the echo identifies the tab however the caller keyed
// it.
func printDeleted(cmd *cobra.Command, cfg *app.Config, t service.DocTab) {
	printOne(cmd, cfg, deleteFields, map[string]any{"deleted_tab": t.TabID, "title": t.Title})
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
