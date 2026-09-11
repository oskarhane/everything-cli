package tabs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSONRendersTabTree(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), seedDocID)

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
	require.Equal(t, seedDocID, svc.listID)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, listFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "t.main", first["tab_id"])
	require.Equal(t, "Main", first["title"])
	require.EqualValues(t, 0, first["index"])
	require.EqualValues(t, 0, first["nesting_level"])
	require.Equal(t, "", first["parent_tab_id"], "a root tab has no parent")
}

// TestListJSONRendersNestedChild pins the depth-first flatten contract at the
// leaf: a nested child tab renders right after its parent, carrying the
// parent's tab id and its own nesting level.
func TestListJSONRendersNestedChild(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), seedDocID)

	rows := cmdtest.DecodeJSON(t, out).([]any)
	child, ok := rows[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "t.child", child["tab_id"])
	require.Equal(t, "Appendix", child["title"])
	require.EqualValues(t, 1, child["nesting_level"])
	require.Equal(t, "t.main", child["parent_tab_id"])

	third, ok := rows[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "t.second", third["tab_id"])
	require.EqualValues(t, 0, third["nesting_level"])
}

func TestListTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"), seedDocID)

	// go-pretty StyleLight upper-cases the headers.
	for _, header := range []string{"TAB_ID", "TITLE", "INDEX", "NESTING_LEVEL", "PARENT_TAB_ID"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "Appendix")
	require.Contains(t, out, "t.main")
}

// TestListToonRendersTabTree pins the TOON render of the flattened tree:
// rows are comma-joined under the key header (keys alphabetize through the
// JSON round-trip), and the nested child row carries its parent's tab id in
// the parent_tab_id column.
func TestListToonRendersTabTree(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "toon"), seedDocID)

	require.Contains(t, out, "{index,nesting_level,parent_tab_id,tab_id,title}:")
	require.Contains(t, out, "\n  0,1,t.main,t.child,Appendix\n")
	require.Contains(t, out, "t.second")
}

// TestListEmptyRendersZeroRows covers the nil-slice contract: a document
// with no tabs renders as zero rows in every format, never panicking.
func TestListEmptyRendersZeroRows(t *testing.T) {
	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			svc := &fakeDocService{tabs: nil}
			out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, format), seedDocID)

			require.NotEmpty(t, out)
			if format == "json" {
				require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
			}
		})
	}
}

func TestListPassesDocID(t *testing.T) {
	svc := &fakeDocService{tabs: seedTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), seedDocID)

	require.Equal(t, seedDocID, svc.listID)
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{listErr: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"), seedDocID)

	require.ErrorIs(t, err, errAPI)
}

func TestListRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}

// errAPI stands in for a Google API failure any leaf must propagate.
var errAPI = errors.New("googleapi: Error 403: access denied")
