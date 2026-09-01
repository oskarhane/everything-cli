package sheets

import (
	"github.com/spf13/cobra"

	sheets "google.golang.org/api/sheets/v4"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/sheets/values"
)

// sheetFields is the sheet-tab row field order for `sheets get` output; the
// same names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var sheetFields = []string{"sheet_id", "title", "index", "row_count", "col_count", "header"}

// sheetRow maps one sheet tab (plus its best-effort header row) to its output
// row. Grid counts render "" for a sheet without grid properties instead of
// a misleading 0; header keeps the cells for JSON/TOON and is compacted for
// table cells.
func sheetRow(p *sheets.SheetProperties, header []any) map[string]any {
	rowCount, colCount := int64(0), int64(0)
	if p.GridProperties != nil {
		rowCount, colCount = p.GridProperties.RowCount, p.GridProperties.ColumnCount
	}
	return map[string]any{
		"sheet_id":  p.SheetId,
		"title":     p.Title,
		"index":     p.Index,
		"row_count": gridCount(rowCount),
		"col_count": gridCount(colCount),
		"header":    orEmpty(header),
	}
}

// orEmpty renders a nil header row as an empty array, never null.
func orEmpty(header []any) []any {
	if header == nil {
		return []any{}
	}
	return header
}

// gridCount renders a grid dimension, or "" when unset (0).
func gridCount(n int64) any {
	if n == 0 {
		return ""
	}
	return n
}

// printSheets renders zero or more sheet tabs: a JSON/TOON array, or a table
// with one row per tab, in the resolved output format.
func printSheets(cmd *cobra.Command, cfg *app.Config, rows []map[string]any) {
	compacted := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		compacted = append(compacted, compactHeader(row))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), sheetFields, rows, compacted)
}

// compactHeader copies row with header joined to a single string; JSON and
// TOON keep the array. The header renders empty when it is not a []any row.
func compactHeader(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		if k == "header" {
			if cells, ok := v.([]any); ok {
				v = values.JoinCells(cells)
			}
		}
		out[k] = v
	}
	return out
}
