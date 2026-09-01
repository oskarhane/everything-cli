package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestCreateRequiresTeamAndTitle(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"))
	require.Error(t, err)

	_, err = cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "--team", "team_1")
	require.Error(t, err, "--title is required even though the API marks it nullable")
}

func TestCreatePassesAllFlags(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--team", "team_1", "--title", "Fix login redirect",
		"--description", "details", "--assignee", "user_1", "--state", "state_1")

	require.Equal(t, "team_1", svc.created.TeamID)
	require.Equal(t, "Fix login redirect", svc.created.Title)
	require.Equal(t, "details", svc.created.Description)
	require.Equal(t, "user_1", svc.created.AssigneeID)
	require.Equal(t, "state_1", svc.created.StateID)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ENG-4", m["identifier"])
}
