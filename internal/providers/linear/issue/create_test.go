package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateRequiresTeamAndTitle(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newStateLeafCmd(newCreateCmd, svc, "json"))
	require.Error(t, err)

	_, err = cmdtest.RunCmdErr(t, newStateLeafCmd(newCreateCmd, svc, "json"), "--team", "team_1")
	require.Error(t, err, "--title is required even though the API marks it nullable")
}

func TestCreatePassesAllFlags(t *testing.T) {
	svc := &fakeService{states: seedStates()}
	out := cmdtest.RunCmd(t, newStateLeafCmd(newCreateCmd, svc, "json"),
		"--team", "team_1", "--title", "Fix login redirect",
		"--description", "details", "--assignee", "user_1", "--state", "in progress")

	require.Equal(t, "team_1", svc.created.TeamID)
	require.Equal(t, "Fix login redirect", svc.created.Title)
	require.Equal(t, "details", svc.created.Description)
	require.Equal(t, "user_1", svc.created.AssigneeID)
	require.Equal(t, "state_2", svc.created.StateID)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ENG-4", m["identifier"])
}

// TestCreateResolvesStateNameWithinTeam pins that --state names are looked
// up in the team named by the required --team flag.
func TestCreateResolvesStateNameWithinTeam(t *testing.T) {
	svc := &fakeService{states: seedStates()}

	cmdtest.RunCmd(t, newStateLeafCmd(newCreateCmd, svc, "json"),
		"--team", "team_7", "--title", "Fix login redirect", "--state", "Done")

	require.Equal(t, "team_7", svc.listStatesTeam)
	require.Equal(t, 1, svc.listStatesCalls)
	require.Equal(t, "state_3", svc.created.StateID)
}

func TestCreateUUIDStateSkipsLookup(t *testing.T) {
	svc := &fakeService{}

	cmdtest.RunCmd(t, newStateLeafCmd(newCreateCmd, svc, "json"),
		"--team", "team_1", "--title", "Fix login redirect", "--state", stateUUID)

	require.Zero(t, svc.listStatesCalls)
	require.Equal(t, stateUUID, svc.created.StateID)
}
