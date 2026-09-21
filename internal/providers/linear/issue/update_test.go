package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestUpdateRequiresAChange(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	_, err := cmdtest.RunCmdErr(t, newStateLeafCmd(newUpdateCmd, svc, "json"), "ENG-1")
	require.ErrorContains(t, err, "nothing to update")
}

func TestUpdatePassesPositionalIDAndFlags(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}, states: seedStates()}
	out := cmdtest.RunCmd(t, newStateLeafCmd(newUpdateCmd, svc, "json"),
		"ENG-1", "--title", "Retitled", "--state", "In Progress")

	require.Equal(t, "ENG-1", svc.updatedID)
	require.Equal(t, "Retitled", svc.updated.Title)
	require.Equal(t, "state_2", svc.updated.StateID)
	require.Empty(t, svc.updated.Description)
	require.Empty(t, svc.updated.AssigneeID)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Retitled", m["title"])
}

// TestUpdateResolvesStateNameWithinIssueTeam pins that a named --state is
// resolved in the team read off the issue, after fetching the issue.
func TestUpdateResolvesStateNameWithinIssueTeam(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}, states: seedStates()}

	cmdtest.RunCmd(t, newStateLeafCmd(newUpdateCmd, svc, "json"), "ENG-1", "--state", "done")

	require.Equal(t, "team_1", svc.listStatesTeam)
	require.Equal(t, "state_3", svc.updated.StateID)
	require.Equal(t, []string{"GetIssue", "ListStates"}, svc.calls,
		"the issue (and its team) must be fetched before name resolution")
}

func TestUpdateStateNameMissingTeamFails(t *testing.T) {
	issue := seedIssue()
	issue.Team = nil
	svc := &fakeService{issues: []service.Issue{issue}, states: seedStates()}

	_, err := cmdtest.RunCmdErr(t, newStateLeafCmd(newUpdateCmd, svc, "json"), "ENG-1", "--state", "Done")

	require.ErrorContains(t, err, "has no team")
	require.Zero(t, svc.listStatesCalls)
}

func TestUpdateUUIDStateSkipsLookup(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newStateLeafCmd(newUpdateCmd, svc, "json"), "ENG-1", "--state", stateUUID)

	require.Empty(t, svc.calls, "a UUID --state needs neither GetIssue nor ListStates")
	require.Equal(t, stateUUID, svc.updated.StateID)
}
