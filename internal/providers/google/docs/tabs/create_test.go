package tabs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateEchoesNewTabID(t *testing.T) {
	svc := &fakeDocService{addTabID: "t.new"}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, ""), seedDocID, "--title", "Appendix")

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, createFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "t.new", detail["tab_id"])
	require.Equal(t, "Appendix", detail["title"])
}

func TestCreateRecordsCall(t *testing.T) {
	svc := &fakeDocService{addTabID: "t.new"}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, ""), seedDocID, "--title", "Appendix")

	require.Equal(t, 1, svc.dials)
	require.Equal(t, seedDocID, svc.addDocID)
	require.Equal(t, "Appendix", svc.addTitle)
}

func TestCreateRequiresTitle(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, ""), seedDocID)

	require.Contains(t, err.Error(), "--title is required")
	require.Zero(t, svc.dials, "no dial on a missing title")
	require.Zero(t, svc.addDocID, "no add on a missing title")
}

func TestCreatePropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{addErr: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, ""), seedDocID, "--title", "Appendix")

	require.ErrorIs(t, err, errAPI)
}

func TestCreateTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeDocService{addTabID: "t.new"}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "table"), seedDocID, "--title", "Appendix")

	for _, header := range []string{"TAB_ID", "TITLE"} {
		require.Contains(t, out, header)
	}
}
