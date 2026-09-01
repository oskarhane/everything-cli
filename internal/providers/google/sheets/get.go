package sheets

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	sheets "google.golang.org/api/sheets/v4"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// newGetCmd returns `sheets get`: one row per sheet tab, with the tab's grid
// sizes and a best-effort header (the tab's first row, read from A1 through
// the last column of its grid). A tab that yields no header row (empty grid,
// unreadable range) still renders — header is simply empty.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[sheetMetaService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <spreadsheet-id>",
		Short: "Show a spreadsheet's sheet tabs with their grid sizes",
		Example: `# List the sheet tabs as JSON
google-cli sheets get 1AbCdEfGh --format json

# Show the same tabs as a table
google-cli sheets get 1AbCdEfGh --format table

# Get a tab by exact title for scripting
google-cli sheets get 1AbCdEfGh --format json | jq '.sheets[] | select(.title=="Budget")'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			spreadsheet, err := svc.GetSpreadsheet(cmd.Context(), id)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(spreadsheet.Sheets))
			for _, sheet := range spreadsheet.Sheets {
				rows = append(rows, sheetRow(sheet.Properties, headerRow(cmd.Context(), svc, id, sheet.Properties)))
			}
			printSheets(cmd, cfg, rows)
			return nil
		},
	}
	return cmd
}

// headerRow reads the tab's first row via values.get on 'title'!A1:{col},
// quoting the title and doubling embedded quotes so the A1 reference stays
// one range. Failure is non-fatal: the header renders empty.
func headerRow(ctx context.Context, svc sheetMetaService, id string, props *sheets.SheetProperties) []any {
	cols := gridCols(props)
	if cols <= 0 {
		return nil
	}
	rangeA1 := fmt.Sprintf("'%s'!A1:%s", service.DoubleSingleQuotes(props.Title), colLetter(cols))
	vals, err := svc.GetValues(ctx, id, rangeA1)
	if err != nil || len(vals) == 0 {
		return nil
	}
	return vals[0]
}

// gridCols returns the tab's grid column count, 0 when the tab has no grid
// properties (an object sheet) — no generated getter exists on the type.
func gridCols(props *sheets.SheetProperties) int64 {
	if props.GridProperties == nil {
		return 0
	}
	return props.GridProperties.ColumnCount
}

// colLetter converts a 1-based column number to A1 letters (1 → "A", 28 → "AB").
func colLetter(n int64) string {
	letters := ""
	for n > 0 {
		letters = string(rune('A'+(n-1)%26)) + letters
		n = (n - 1) / 26
	}
	return letters
}
