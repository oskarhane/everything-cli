package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateRequiresTeamAndTitle(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"))
	require.Error(t, err)

	_, err = cmdtest.RunCmdErr(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"), "--team", "team_1")
	require.Error(t, err, "--title is required even though the API marks it nullable")
}

func TestCreatePassesAllFlags(t *testing.T) {
	svc := &fakeService{states: seedStates()}
	out := cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_1", "--title", "Fix login redirect",
		"--description", "details", "--assignee", "user_1", "--state", "in progress",
		"--project", "proj_1")

	require.Equal(t, "team_1", svc.created.TeamID)
	require.Equal(t, "Fix login redirect", svc.created.Title)
	require.Equal(t, "details", svc.created.Description)
	require.Equal(t, "user_1", svc.created.AssigneeID)
	require.Equal(t, "state_2", svc.created.StateID)
	require.Equal(t, "proj_1", svc.created.ProjectID)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ENG-4", m["identifier"])
}

// TestCreateResolvesStateNameWithinTeam pins that --state names are looked
// up in the team named by the required --team flag.
func TestCreateResolvesStateNameWithinTeam(t *testing.T) {
	svc := &fakeService{states: seedStates()}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_7", "--title", "Fix login redirect", "--state", "Done")

	require.Equal(t, "team_7", svc.listStatesTeam)
	require.Equal(t, 1, svc.listStatesCalls)
	require.Equal(t, "state_3", svc.created.StateID)
}

func TestCreateUUIDStateSkipsLookup(t *testing.T) {
	svc := &fakeService{}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_1", "--title", "Fix login redirect", "--state", stateUUID)

	require.Zero(t, svc.listStatesCalls)
	require.Equal(t, stateUUID, svc.created.StateID)
}

// TestCreateResolvesParentIdentifier pins that a human --parent identifier
// resolves to the parent's UUID via GetIssue.
func TestCreateResolvesParentIdentifier(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedParentIssue()}}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_1", "--title", "Sub-task", "--parent", "ENG-9")

	require.Equal(t, "ENG-9", svc.gotID)
	require.Equal(t, "issue_9", svc.created.ParentID)
}

func TestCreateUUIDParentSkipsLookup(t *testing.T) {
	svc := &fakeService{}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_1", "--title", "Sub-task", "--parent", stateUUID)

	require.Empty(t, svc.calls, "a UUID --parent needs no GetIssue")
	require.Equal(t, stateUUID, svc.created.ParentID)
}

// TestCreateResolvesLabelsByName pins that --labels names resolve against
// the team named by --team, while UUID entries pass through untouched.
func TestCreateResolvesLabelsByName(t *testing.T) {
	svc := &fakeService{}
	wf := &writeFakes{labels: seedLabels()}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, wf, "json"),
		"--team", "team_1", "--title", "Fix login redirect", "--labels", "bug, "+stateUUID)

	require.Equal(t, "team_1", wf.listLabelsTeam)
	require.Equal(t, []string{"label_1", stateUUID}, svc.created.LabelIDs)
}

func TestCreateUnknownLabelErrors(t *testing.T) {
	svc := &fakeService{}
	wf := &writeFakes{labels: seedLabels()}

	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newCreateCmd, svc, wf, "json"),
		"--team", "team_1", "--title", "Fix login redirect", "--labels", "Nope")

	require.ErrorContains(t, err, `unknown label "Nope"`)
}

// TestCreatePriorityNameMapping pins the urgent/high/medium/low/none →
// 1/2/3/4/0 mapping plus digit passthrough.
func TestCreatePriorityNameMapping(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{"urgent", 1},
		{"high", 2},
		{"medium", 3},
		{"low", 4},
		{"none", 0},
		{"3", 3},
	}
	for _, tc := range cases {
		svc := &fakeService{}

		cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
			"--team", "team_1", "--title", "Fix login redirect", "--priority", tc.value)

		require.Equal(t, tc.want, svc.created.Priority, tc.value)
	}
}

func TestCreateInvalidPriorityErrors(t *testing.T) {
	svc := &fakeService{}

	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_1", "--title", "Fix login redirect", "--priority", "5")

	require.ErrorContains(t, err, "invalid priority")
}

// TestCreateResolvesCycleAndMilestoneByName pins that --cycle resolves
// within --team and --milestone within --project.
func TestCreateResolvesCycleAndMilestoneByName(t *testing.T) {
	svc := &fakeService{}
	wf := &writeFakes{cycles: seedCycles(), milestones: seedMilestones()}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, wf, "json"),
		"--team", "team_1", "--title", "Fix login redirect",
		"--project", "proj_1", "--cycle", "Sprint 12", "--milestone", "beta")

	require.Equal(t, "team_1", wf.listCyclesTeam)
	require.Equal(t, "proj_1", wf.listMilestonesProject)
	require.Equal(t, "cycle_1", svc.created.CycleID)
	require.Equal(t, "milestone_1", svc.created.ProjectMilestoneID)
}

func TestCreateMilestoneNameNeedsProject(t *testing.T) {
	svc := &fakeService{}
	wf := &writeFakes{milestones: seedMilestones()}

	_, err := cmdtest.RunCmdErr(t, newWriteLeafCmd(newCreateCmd, svc, wf, "json"),
		"--team", "team_1", "--title", "Fix login redirect", "--milestone", "Beta")

	require.ErrorContains(t, err, "pass --project")
	require.Empty(t, wf.listMilestonesProject, "no project means no milestone lookup")
}

func TestCreatePassesDueDateAndEstimate(t *testing.T) {
	svc := &fakeService{}

	cmdtest.RunCmd(t, newWriteLeafCmd(newCreateCmd, svc, &writeFakes{}, "json"),
		"--team", "team_1", "--title", "Fix login redirect",
		"--due-date", "2026-10-01", "--estimate", "3")

	require.Equal(t, "2026-10-01", svc.created.DueDate)
	require.Equal(t, 3, svc.created.Estimate)
}
