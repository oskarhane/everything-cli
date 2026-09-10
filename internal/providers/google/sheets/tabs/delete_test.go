package tabs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteEchoesDeletedTab(t *testing.T) {
	svc := &fakeTabService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, ""), seedSpreadsheetID, "--tab", "Notes")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, deleteFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Notes", detail["deleted_tab"])
}

func TestDeleteRecordsExactTitle(t *testing.T) {
	svc := &fakeTabService{}
	cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, ""), seedSpreadsheetID, "--tab", "Notes")

	require.Equal(t, 1, svc.deleteCalls)
	require.Equal(t, seedSpreadsheetID, svc.deleteSpreadsheet)
	require.Equal(t, "Notes", svc.deleteTab)
}

func TestDeleteRequiresTabTitle(t *testing.T) {
	svc := &fakeTabService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedSpreadsheetID)

	require.Contains(t, err.Error(), "--tab is required")
	require.Equal(t, 0, svc.deleteCalls, "no write without the tab title")
}

func TestDeletePropagatesUnknownTitleError(t *testing.T) {
	svc := &fakeTabService{deleteErr: errors.New(`spreadsheet sheet_1 has no sheet named "budget"`)}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedSpreadsheetID, "--tab", "budget")

	require.Contains(t, err.Error(), `"budget"`)
	require.Equal(t, 1, svc.deleteCalls, "the lookup failure surfaces from the service call")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeTabService{deleteErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedSpreadsheetID, "--tab", "Notes")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
