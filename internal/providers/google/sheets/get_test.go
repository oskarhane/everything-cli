package sheets

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestGetJSONOneRowPerTab(t *testing.T) {
	svc := &fakeSheetService{spreadsheet: seedSpreadsheet()}
	out := cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, sheetFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.EqualValues(t, 0, first["sheet_id"])
	require.Equal(t, "Budget", first["title"])
	require.EqualValues(t, 0, first["index"])
	require.EqualValues(t, 100, first["row_count"])
	require.EqualValues(t, 3, first["col_count"])
}

func TestGetSingleTabRendersOneObject(t *testing.T) {
	spreadsheet := seedSpreadsheet()
	spreadsheet.Sheets = spreadsheet.Sheets[:1]
	svc := &fakeSheetService{spreadsheet: spreadsheet}
	out := cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	require.Equal(t, "Budget", row["title"])
}

func TestGetTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeSheetService{spreadsheet: seedSpreadsheet()}
	out := cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "table"), "sheet_1")

	for _, header := range []string{"SHEET_ID", "TITLE", "INDEX", "ROW_COUNT", "COL_COUNT", "HEADER"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "Budget")
}

func TestGetHeaderComesFromValuesGetOnTabRange(t *testing.T) {
	svc := &fakeSheetService{
		spreadsheet: seedSpreadsheet(),
		values:      map[string][][]any{"'Budget'!A1:C": {{"Name", "Amount", "Note"}}},
	}
	out := cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	require.Contains(t, svc.valuesRanges, "'Budget'!A1:C", "header reads the tab's full first row")
	require.Equal(t, "sheet_1", svc.valuesID)

	row, _ := cmdtest.DecodeJSON(t, out).([]any)[0].(map[string]any)
	require.Equal(t, []any{"Name", "Amount", "Note"}, row["header"])
}

func TestGetHeaderFailureIsNotFatal(t *testing.T) {
	svc := &fakeSheetService{spreadsheet: seedSpreadsheet(), valuesErr: errors.New("googleapi: Error 403")}
	out := cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, rows, 2)
	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{}, first["header"], "an unreadable header renders empty, not an error")
}

func TestGetHeaderRangeSkipsTabsWithoutGrid(t *testing.T) {
	spreadsheet := seedSpreadsheet()
	spreadsheet.Sheets[0].Properties.GridProperties = nil
	svc := &fakeSheetService{spreadsheet: spreadsheet}
	cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	// Only the 2-column tab is queried; the object sheet is not.
	require.Equal(t, []string{"'Notes'!A1:B"}, svc.valuesRanges)
}

func TestGetHeaderEscapesQuotesInTitle(t *testing.T) {
	spreadsheet := seedSpreadsheet()
	spreadsheet.Sheets[0].Properties.Title = "O'Brien's"
	svc := &fakeSheetService{spreadsheet: spreadsheet}
	cmdtest.RunCmd(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	require.Contains(t, svc.valuesRanges, "'O''Brien''s'!A1:C")
}

func TestGetPropagatesMetadataError(t *testing.T) {
	svc := &fakeSheetService{metaErr: errors.New("googleapi: Error 404: not found")}
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"), "sheet_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeSheetService{spreadsheet: seedSpreadsheet()}
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[sheetMetaService](newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
