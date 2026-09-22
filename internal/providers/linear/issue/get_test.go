package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestGetJSON(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "ENG-1")

	require.Equal(t, "ENG-1", svc.gotID)
	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "issue_1", m["id"])
	require.Equal(t, "Fix login redirect", m["title"])
	require.Equal(t, "https://linear.app/x/issue/ENG-1", m["url"])
	require.Contains(t, m, "created_at")
}

// TestGetDetailFieldsJSON checks the get leaf surfaces the detail-only
// fields in JSON output when populated.
func TestGetDetailFieldsJSON(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{detailIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "ENG-1")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"id": "issue_0", "identifier": "ENG-0", "title": "Parent epic"}, m["parent"])
	require.Equal(t, []any{
		map[string]any{"id": "issue_2", "identifier": "ENG-2", "title": "Child one"},
		map[string]any{"id": "issue_3", "identifier": "ENG-3", "title": "Child two"},
	}, m["children"])
	require.Equal(t, float64(2), m["priority"])
	require.Equal(t, "High", m["priority_label"])
	require.Equal(t, "2026-09-30", m["due_date"])
	require.Equal(t, float64(3), m["estimate"])
	require.Equal(t, map[string]any{"id": "cycle_1", "name": "Cycle 12"}, m["cycle"])
	require.Equal(t, map[string]any{"id": "ms_1", "name": "v2.0"}, m["milestone"])
}

// TestGetDetailFieldsAbsentJSON checks absent detail-only fields do not
// surface as empty keys in JSON output.
func TestGetDetailFieldsAbsentJSON(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "ENG-1")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.NotContains(t, m, "parent")
	require.NotContains(t, m, "children")
	require.NotContains(t, m, "priority_label")
	require.NotContains(t, m, "due_date")
	require.NotContains(t, m, "estimate")
	require.NotContains(t, m, "cycle")
	require.NotContains(t, m, "milestone")
}

// TestGetDetailFieldsTable checks the detail table gains the new columns
// (upper-case headers) and renders parent/children as identifiers.
func TestGetDetailFieldsTable(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{detailIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "ENG-1")

	for _, header := range []string{"PARENT", "CHILDREN", "PRIORITY", "DUE_DATE", "ESTIMATE", "CYCLE", "MILESTONE"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "ENG-0")
	require.Contains(t, out, "ENG-2, ENG-3")
	require.Contains(t, out, "High")
	require.Contains(t, out, "2026-09-30")
	require.Contains(t, out, "Cycle 12")
	require.Contains(t, out, "v2.0")
}

func TestGetUnknownIssueFails(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "ENG-999")
	require.ErrorContains(t, err, `issue "ENG-999" not found`)
}
