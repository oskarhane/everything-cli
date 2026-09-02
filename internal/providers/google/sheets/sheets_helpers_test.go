package sheets

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	sheets "google.golang.org/api/sheets/v4"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeSheetService is the hermetic double for both surfaces `sheets get`
// needs (spreadsheet metadata + values reads for headers): it serves a seeded
// spreadsheet and records every call. The embedded nil service interfaces
// satisfy any surface the parent hands down that these leaves never call.
type fakeSheetService struct {
	service.SheetService
	service.SheetValuesService

	spreadsheet *sheets.Spreadsheet // served by GetSpreadsheet
	metaErr     error               // when set, GetSpreadsheet fails
	valuesErr   error               // when set, every GetValues fails
	values      map[string][][]any  // served by GetValues, keyed by A1 range

	metaID       string   // last GetSpreadsheet id
	valuesID     string   // last GetValues spreadsheet id
	valuesRange  string   // last GetValues A1 range
	valuesRanges []string // every GetValues A1 range, in call order
}

func (f *fakeSheetService) GetSpreadsheet(_ context.Context, id string) (*sheets.Spreadsheet, error) {
	f.metaID = id
	if f.metaErr != nil {
		return nil, f.metaErr
	}
	return f.spreadsheet, nil
}

func (f *fakeSheetService) GetValues(_ context.Context, id, a1Range string) ([][]any, error) {
	f.valuesID, f.valuesRange = id, a1Range
	f.valuesRanges = append(f.valuesRanges, a1Range)
	if f.valuesErr != nil {
		return nil, f.valuesErr
	}
	return f.values[a1Range], nil
}

// newSheetCmd builds a leaf against a fake service, ready to execute.
func newSheetCmd[T any](build func(*app.Config, service.Dialer[T]) *cobra.Command, svc T, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), func(context.Context) (T, error) { return svc, nil })
}

// seedSpreadsheet returns a spreadsheet with two grid tabs: one with a
// 3-column grid, one with 2.
func seedSpreadsheet() *sheets.Spreadsheet {
	return &sheets.Spreadsheet{
		SpreadsheetId: "sheet_1",
		Sheets: []*sheets.Sheet{
			{Properties: &sheets.SheetProperties{
				SheetId: 0, Title: "Budget", Index: 0,
				GridProperties: &sheets.GridProperties{RowCount: 100, ColumnCount: 3},
			}},
			{Properties: &sheets.SheetProperties{SheetId: 1, Title: "Notes", Index: 1,
				GridProperties: &sheets.GridProperties{RowCount: 50, ColumnCount: 2},
			}},
		},
	}
}
