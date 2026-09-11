package tabs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestRenameEchoesOldAndNewTitle(t *testing.T) {
	svc := &fakeTabService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newRenameCmd, svc, ""), seedSpreadsheetID,
		"--tab", "Notes", "--title", "Archive")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, renameFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Notes", detail["old_title"])
	require.Equal(t, "Archive", detail["new_title"])
}

func TestRenameRecordsExactTitles(t *testing.T) {
	svc := &fakeTabService{}
	cmdtest.RunCmd(t, newLeafCmd(newRenameCmd, svc, ""), seedSpreadsheetID,
		"--tab", "Notes", "--title", "Archive")

	require.Equal(t, 1, svc.renameCalls)
	require.Equal(t, seedSpreadsheetID, svc.renameSpreadsheet)
	require.Equal(t, "Notes", svc.renameOld)
	require.Equal(t, "Archive", svc.renameNew)
}

func TestRenameRequiresTitle(t *testing.T) {
	svc := &fakeTabService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedSpreadsheetID, "--tab", "Notes")

	require.Contains(t, err.Error(), "--title is required")
	require.Zero(t, svc.renameCalls, "no rename on a missing title")
}

func TestRenameRequiresTabTitle(t *testing.T) {
	svc := &fakeTabService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedSpreadsheetID, "--title", "Archive")

	require.Contains(t, err.Error(), "--tab is required")
	require.Zero(t, svc.renameCalls, "no rename on a missing current title")
}

func TestRenamePropagatesUnknownTitleError(t *testing.T) {
	svc := &fakeTabService{renameErr: errors.New(`spreadsheet sheet_1 has no sheet named "notes"`)}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedSpreadsheetID,
		"--tab", "notes", "--title", "Archive")

	require.Contains(t, err.Error(), `"notes"`)
}

func TestRenamePropagatesAPIError(t *testing.T) {
	svc := &fakeTabService{renameErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedSpreadsheetID,
		"--tab", "Notes", "--title", "Archive")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
