package tabs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateEchoesNewSheetID(t *testing.T) {
	svc := &fakeTabService{addID: 42}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, ""), seedSpreadsheetID, "--title", "Forecast")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, createFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.EqualValues(t, 42, detail["sheet_id"])
	require.Equal(t, "Forecast", detail["title"])
}

func TestCreateRecordsCall(t *testing.T) {
	svc := &fakeTabService{addID: 7}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, ""), seedSpreadsheetID, "--title", "Forecast")

	require.Equal(t, 1, svc.addCalls)
	require.Equal(t, seedSpreadsheetID, svc.addSpreadsheet)
	require.Equal(t, "Forecast", svc.addTitle)
}

func TestCreateRequiresTitle(t *testing.T) {
	svc := &fakeTabService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, ""), seedSpreadsheetID)

	require.Contains(t, err.Error(), "--title is required")
	require.Zero(t, svc.addCalls, "no add on a missing title")
}

func TestCreatePropagatesAPIError(t *testing.T) {
	svc := &fakeTabService{addErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, ""), seedSpreadsheetID, "--title", "Forecast")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}

func TestCreateTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeTabService{addID: 42}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "table"), seedSpreadsheetID, "--title", "Forecast")

	for _, header := range []string{"SHEET_ID", "TITLE"} {
		require.Contains(t, out, header)
	}
}
