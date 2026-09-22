package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestUpdateRequiresAChange(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"), "ENG-1")
	require.ErrorContains(t, err, "nothing to update")
}

func TestUpdatePassesPositionalIDAndFlags(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}, states: seedStates()}
	out := cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
		"ENG-1", "--title", "Retitled", "--state", "In Progress", "--project", "proj_1")

	require.Equal(t, "ENG-1", svc.updatedID)
	require.Equal(t, "Retitled", svc.updated.Title)
	require.Equal(t, "state_2", svc.updated.StateID)
	require.Equal(t, "proj_1", svc.updated.ProjectID)
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

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"), "ENG-1", "--state", "done")

	require.Equal(t, "team_1", svc.listStatesTeam)
	require.Equal(t, "state_3", svc.updated.StateID)
	require.Equal(t, []string{"GetIssue", "ListStates"}, svc.calls,
		"the issue (and its team) must be fetched before name resolution")
}

func TestUpdateStateNameMissingTeamFails(t *testing.T) {
	issue := seedIssue()
	issue.Team = nil
	svc := &fakeService{issues: []service.Issue{issue}, states: seedStates()}

	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"), "ENG-1", "--state", "Done")

	require.ErrorContains(t, err, "has no team")
	require.Zero(t, svc.listStatesCalls)
}

func TestUpdateProjectOnlyCountsAsAChange(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"), "ENG-1", "--project", "proj_1")

	require.Equal(t, "proj_1", svc.updated.ProjectID)
	require.Empty(t, svc.calls, "a --project update needs no state lookup")
}

func TestUpdateUUIDStateSkipsLookup(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"), "ENG-1", "--state", stateUUID)

	require.Empty(t, svc.calls, "a UUID --state needs neither GetIssue nor ListStates")
	require.Equal(t, stateUUID, svc.updated.StateID)
}

// TestUpdateParentByIdentifier pins that a human --parent identifier
// resolves to the parent's UUID via GetIssue.
func TestUpdateParentByIdentifier(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue(), seedParentIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
		"ENG-1", "--parent", "ENG-9")

	require.NotNil(t, svc.updated.ParentID)
	require.Equal(t, "issue_9", *svc.updated.ParentID)
}

// TestUpdateParentClear pins that --parent "" sends an un-parent (null on
// the wire) and counts as a change for the nothing-to-update check.
func TestUpdateParentClear(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
		"ENG-1", "--parent", "")

	require.NotNil(t, svc.updated.ParentID)
	require.Empty(t, *svc.updated.ParentID)
	require.Empty(t, svc.calls, "clearing the parent needs no GetIssue")
}

// TestUpdateLabelsByName pins that --labels names resolve in the team read
// off the issue, after fetching the issue.
func TestUpdateLabelsByName(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	wf := &writeFakes{labels: seedLabels()}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, wf, "json"),
		"ENG-1", "--labels", "bug, Feature")

	require.NotNil(t, svc.updated.LabelIDs)
	require.Equal(t, []string{"label_1", "label_2"}, *svc.updated.LabelIDs)
	require.Equal(t, "team_1", wf.listLabelsTeam)
	require.Equal(t, []string{"GetIssue", "ListTeamLabels"}, svc.calls,
		"the issue (and its team) must be fetched before label resolution")
}

// TestUpdateLabelsClear pins that --labels "" clears every label without
// any lookup.
func TestUpdateLabelsClear(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
		"ENG-1", "--labels", "")

	require.NotNil(t, svc.updated.LabelIDs)
	require.Empty(t, *svc.updated.LabelIDs)
	require.Empty(t, svc.calls, "clearing labels needs neither GetIssue nor ListTeamLabels")
}

// TestUpdatePriorityNameMapping pins the name→int mapping on update,
// including none → 0 (which only update can send, via the pointer).
func TestUpdatePriorityNameMapping(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{"urgent", 1},
		{"high", 2},
		{"medium", 3},
		{"low", 4},
		{"none", 0},
		{"2", 2},
	}
	for _, tc := range cases {
		svc := &fakeService{issues: []service.Issue{seedIssue()}}

		cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
			"ENG-1", "--priority", tc.value)

		require.NotNil(t, svc.updated.Priority, tc.value)
		require.Equal(t, tc.want, *svc.updated.Priority, tc.value)
	}
}

// TestUpdateDueDateClear pins that --due-date "" clears the due date.
func TestUpdateDueDateClear(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
		"ENG-1", "--due-date", "")

	require.NotNil(t, svc.updated.DueDate)
	require.Empty(t, *svc.updated.DueDate)
	require.Empty(t, svc.calls, "clearing the due date needs no GetIssue")
}

// TestUpdateResolvesCycleWithinIssueTeam pins that a named --cycle resolves
// in the team read off the issue, after fetching the issue.
func TestUpdateResolvesCycleWithinIssueTeam(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	wf := &writeFakes{cycles: seedCycles()}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, wf, "json"),
		"ENG-1", "--cycle", "Sprint 12")

	require.Equal(t, "team_1", wf.listCyclesTeam)
	require.NotNil(t, svc.updated.CycleID)
	require.Equal(t, "cycle_1", *svc.updated.CycleID)
	require.Equal(t, []string{"GetIssue", "ListCycles"}, svc.calls,
		"the issue (and its team) must be fetched before cycle resolution")
}

// TestUpdateResolvesMilestoneWithinProject pins that a named --milestone
// resolves within the project named by --project.
func TestUpdateResolvesMilestoneWithinProject(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	wf := &writeFakes{milestones: seedMilestones()}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, wf, "json"),
		"ENG-1", "--project", "proj_1", "--milestone", "beta")

	require.Equal(t, "proj_1", wf.listMilestonesProject)
	require.NotNil(t, svc.updated.ProjectMilestoneID)
	require.Equal(t, "milestone_1", *svc.updated.ProjectMilestoneID)
}

// TestUpdateMilestoneNameNeedsProject pins the clear error when a milestone
// name has no project to resolve in: GetIssue's payload does not carry the
// issue's project, so --project must come along.
func TestUpdateMilestoneNameNeedsProject(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	wf := &writeFakes{milestones: seedMilestones()}

	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newUpdateCmd, svc, wf, "json"),
		"ENG-1", "--milestone", "Beta")

	require.ErrorContains(t, err, "pass --project")
	require.Empty(t, svc.calls, "a project-less milestone name fails before any fetch")
}

func TestUpdateEstimateOnlyCountsAsAChange(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newUpdateCmd, svc, &writeFakes{}, "json"),
		"ENG-1", "--estimate", "5")

	require.NotNil(t, svc.updated.Estimate)
	require.Equal(t, 5, *svc.updated.Estimate)
}
