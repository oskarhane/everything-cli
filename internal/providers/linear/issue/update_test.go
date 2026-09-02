package issue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestUpdateRequiresAChange(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), "ENG-1")
	require.ErrorContains(t, err, "nothing to update")
}

func TestUpdatePassesPositionalIDAndFlags(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"),
		"ENG-1", "--title", "Retitled", "--state", "state_2")

	require.Equal(t, "ENG-1", svc.updatedID)
	require.Equal(t, "Retitled", svc.updated.Title)
	require.Equal(t, "state_2", svc.updated.StateID)
	require.Empty(t, svc.updated.Description)
	require.Empty(t, svc.updated.AssigneeID)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Retitled", m["title"])
}
