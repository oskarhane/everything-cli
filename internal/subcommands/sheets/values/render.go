package values

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// Field orders for the values leaves' output; the same names are the
// snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases the
// headers when rendering.
var (
	valuesGetFields = []string{"row", "values"}
	appendFields    = []string{"updated_range", "updated_rows", "updated_cols"}
	updateFields    = []string{"updated_range", "updated_cells"}
	clearFields     = []string{"cleared_range"}
)

// printValuesGet renders a values.read: JSON/TOON gets {"range", "values"}
// (values kept as the API's typed 2D array); table gets one row per
// spreadsheet row with the cells tab-joined. Control bytes are stripped by
// the output layer, not here.
func printValuesGet(cmd *cobra.Command, cfg *app.Config, a1Range string, vals [][]any) {
	if vals == nil {
		vals = [][]any{}
	}
	rows := make([]map[string]any, 0, len(vals))
	for i, row := range vals {
		rows = append(rows, map[string]any{"row": i + 1, "values": joinCells(row)})
	}
	detail := map[string]any{"range": a1Range, "values": vals}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), valuesGetFields, detail, rows)
}

// printAppend prints the echo after an append: updated range and row/column
// counts.
func printAppend(cmd *cobra.Command, cfg *app.Config, updatedRange string, updatedRows, updatedCols int64) {
	printOne(cmd, cfg, appendFields, map[string]any{
		"updated_range": updatedRange,
		"updated_rows":  updatedRows,
		"updated_cols":  updatedCols,
	})
}

// printUpdate renders the echo after an update: updated range and cell count.
func printUpdate(cmd *cobra.Command, cfg *app.Config, updatedRange string, updatedCells int64) {
	printOne(cmd, cfg, updateFields, map[string]any{
		"updated_range": updatedRange,
		"updated_cells": updatedCells,
	})
}

// printClear renders the echo after a clear.
func printClear(cmd *cobra.Command, cfg *app.Config, clearedRange string) {
	printOne(cmd, cfg, clearFields, map[string]any{"cleared_range": clearedRange})
}

// printOne renders a single output object (a mutation echo): an object in
// JSON/TOON, a one-row table with the same fields.
func printOne(cmd *cobra.Command, cfg *app.Config, fields []string, row map[string]any) {
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), fields, row, []map[string]any{row})
}

// joinCells renders one spreadsheet row for a table cell: scalars as text,
// joined with tabs. Non-strings keep their default formatting.
func joinCells(row []any) string {
	cells := make([]string, 0, len(row))
	for _, cell := range row {
		cells = append(cells, fmt.Sprintf("%v", cell))
	}
	return strings.Join(cells, "\t")
}
