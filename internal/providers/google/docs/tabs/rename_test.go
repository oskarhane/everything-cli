package tabs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestRenameByTabID(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID,
		"--tab", "t.second", "--title", "Archive")

	require.Equal(t, 1, svc.renameCalls)
	require.Equal(t, seedDocID, svc.renameDocID)
	require.Equal(t, "t.second", svc.renameTabID, "the ID key reaches the rename as-is")
	require.Equal(t, "Archive", svc.renameTitle)

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, renameFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Notes", detail["old_title"])
	require.Equal(t, "Archive", detail["new_title"])
}

func TestRenameByExactTitle(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID,
		"--tab", "Notes", "--title", "Archive")

	require.Equal(t, 1, svc.renameCalls)
	require.Equal(t, "t.second", svc.renameTabID, "the title resolves to the tab's ID")
	require.Equal(t, "Archive", svc.renameTitle)
}

func TestRenameUnknownTabErrorsWithZeroWrites(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID,
		"--tab", "Nope", "--title", "Archive")

	require.Contains(t, err.Error(), `no tab with ID or title "Nope"`)
	require.Zero(t, svc.renameCalls, "an unknown key must not rename anything")
}

func TestRenameAmbiguousTitleErrorsWithZeroWrites(t *testing.T) {
	svc := &fakeDocService{tabs: []service.DocTab{
		{TabID: "t.a1", Title: "Archive", Index: 0, NestingLevel: 0},
		{TabID: "t.a2", Title: "Archive", Index: 1, NestingLevel: 0},
	}}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID,
		"--tab", "Archive", "--title", "Whatever")

	require.Contains(t, err.Error(), `tab title "Archive" is ambiguous`)
	require.Zero(t, svc.renameCalls, "an ambiguous key must not rename anything")
}

func TestRenameRequiresTab(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID, "--title", "Archive")

	require.Contains(t, err.Error(), "--tab is required")
	require.Zero(t, svc.dials, "a missing --tab errors before any dial")
	require.Zero(t, svc.renameCalls, "no rename on a missing tab key")
}

func TestRenameRequiresTitle(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID, "--tab", "Notes")

	require.Contains(t, err.Error(), "--title is required")
	require.Zero(t, svc.dials, "a missing --title errors before any dial")
	require.Zero(t, svc.renameCalls, "no rename on a missing title")
}

func TestRenamePropagatesListError(t *testing.T) {
	svc := &fakeDocService{listErr: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID,
		"--tab", "Notes", "--title", "Archive")

	require.ErrorIs(t, err, errAPI)
	require.Zero(t, svc.renameCalls, "a failed resolution must not rename anything")
}

func TestRenamePropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs(), renameErr: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newRenameCmd, svc, ""), seedDocID,
		"--tab", "Notes", "--title", "Archive")

	require.ErrorIs(t, err, errAPI)
	require.Equal(t, 1, svc.renameCalls, "the failure surfaces from the rename call")
}

func TestRenameTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newRenameCmd, svc, "table"), seedDocID,
		"--tab", "Notes", "--title", "Archive")

	for _, header := range []string{"OLD_TITLE", "NEW_TITLE"} {
		require.Contains(t, out, header)
	}
}
