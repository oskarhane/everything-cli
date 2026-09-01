package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/providers/linear/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
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

func TestGetUnknownIssueFails(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "ENG-999")
	require.ErrorContains(t, err, `issue "ENG-999" not found`)
}
