package values

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestClearJSON(t *testing.T) {
	svc := &fakeValuesService{clearReturned: "Sheet1!A1:B2"}
	out := cmdtest.RunCmd(t, newLeafCmd(newClearCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, clearFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Sheet1!A1:B2", detail["cleared_range"])
}

func TestClearPassesRangeThrough(t *testing.T) {
	svc := &fakeValuesService{clearReturned: "Sheet1!B2:B"}
	cmdtest.RunCmd(t, newLeafCmd(newClearCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!B2:B")

	require.Equal(t, seedSpreadsheetID, svc.clearID)
	require.Equal(t, "Sheet1!B2:B", svc.clearRange)
}

func TestClearHasNoForceFlag(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newClearCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:B2", "--force")

	require.Contains(t, err.Error(), "unknown flag: --force")
	require.Empty(t, svc.clearRange, "nothing was cleared")
}

func TestClearRequiresRange(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newClearCmd, svc, "json"), seedSpreadsheetID)

	require.Contains(t, err.Error(), `required flag(s) "range" not set`)
}

func TestClearPropagatesAPIError(t *testing.T) {
	svc := &fakeValuesService{clearErr: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newClearCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestClearTableUpperCasesHeader(t *testing.T) {
	svc := &fakeValuesService{clearReturned: "Sheet1!A1:B2"}
	out := cmdtest.RunCmd(t, newLeafCmd(newClearCmd, svc, "table"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	require.Contains(t, out, "CLEARED_RANGE")
}
