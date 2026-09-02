package values

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestValuesGetJSON(t *testing.T) {
	svc := &fakeValuesService{values: map[string][][]any{
		"Sheet1!A1:B2": {{"name", "amount"}, {"row2", float64(3)}},
	}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, []string{"range", "values"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Sheet1!A1:B2", detail["range"])
	require.Equal(t, []any{
		[]any{"name", "amount"},
		[]any{"row2", float64(3)},
	}, detail["values"])
}

func TestValuesGetTableUpperCasesHeadersAndJoinsCells(t *testing.T) {
	svc := &fakeValuesService{values: map[string][][]any{"Sheet1!A1:B2": {{"Name", "Amount"}, {"a,b", 1.5}}}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	require.Contains(t, out, "ROW")
	require.Contains(t, out, "VALUES")
	// Tab-joined cells render inside one cell; the output layer collapses the
	// tab, so assert both cells appear (not the raw tab join).
	require.Contains(t, out, "a,b")
	require.Contains(t, out, "1.5")
}

func TestValuesGetPassesRangeThrough(t *testing.T) {
	svc := &fakeValuesService{values: map[string][][]any{"A1:B2": {{"a"}}}}
	cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	require.Equal(t, seedSpreadsheetID, svc.getID)
	require.Equal(t, "Sheet1!A1:B2", svc.getRange)
}

func TestValuesGetEmptyRange(t *testing.T) {
	svc := &fakeValuesService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!Z100:Z101")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{}, detail["values"])
}

func TestValuesGetRequiresRange(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), seedSpreadsheetID)

	require.Contains(t, err.Error(), `required flag(s) "range" not set`)
}

func TestValuesGetPropagatesAPIError(t *testing.T) {
	svc := &fakeValuesService{getErr: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:B")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestValuesGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
