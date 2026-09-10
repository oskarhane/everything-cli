package tabs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteByTabID(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID, "--tab", "t.second")

	require.Equal(t, 1, svc.deleteCalls, "the delete goes out once")
	require.Equal(t, seedDocID, svc.deleteDocID)
	require.Equal(t, "t.second", svc.deleteTabID, "the ID key reaches the delete as-is")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, deleteFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "t.second", detail["deleted_tab"])
	require.Equal(t, "Notes", detail["title"])
}

func TestDeleteByExactTitle(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID, "--tab", "Notes")

	require.Equal(t, 1, svc.deleteCalls)
	require.Equal(t, "t.second", svc.deleteTabID, "the title resolves to the tab's ID")
}

func TestDeleteUnknownTabErrorsWithZeroWrites(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID, "--tab", "Nope")

	require.Contains(t, err.Error(), `no tab with ID or title "Nope"`)
	require.Zero(t, svc.deleteCalls, "an unknown key must not delete anything")
}

func TestDeleteAmbiguousTitleErrorsWithZeroWrites(t *testing.T) {
	svc := &fakeDocService{tabs: []service.DocTab{
		{TabID: "t.a1", Title: "Archive", Index: 0, NestingLevel: 0},
		{TabID: "t.a2", Title: "Archive", Index: 1, NestingLevel: 0},
	}}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID, "--tab", "Archive")

	require.Contains(t, err.Error(), `tab title "Archive" is ambiguous`)
	require.Contains(t, err.Error(), "t.a1")
	require.Contains(t, err.Error(), "t.a2")
	require.Zero(t, svc.deleteCalls, "an ambiguous key must not delete anything")
}

func TestDeleteRequiresTab(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID)

	require.Contains(t, err.Error(), "--tab is required")
	require.Zero(t, svc.dials, "a missing --tab errors before any dial")
	require.Zero(t, svc.deleteCalls, "no write without the tab key")
}

func TestDeletePropagatesListError(t *testing.T) {
	svc := &fakeDocService{listErr: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID, "--tab", "Notes")

	require.ErrorIs(t, err, errAPI)
	require.Zero(t, svc.deleteCalls, "a failed resolution must not delete anything")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs(), deleteErr: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, ""), seedDocID, "--tab", "Notes")

	require.ErrorIs(t, err, errAPI)
	require.Equal(t, 1, svc.deleteCalls, "the failure surfaces from the delete call")
}

func TestDeleteTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "table"), seedDocID, "--tab", "Notes")

	for _, header := range []string{"DELETED_TAB", "TITLE"} {
		require.Contains(t, out, header)
	}
}
