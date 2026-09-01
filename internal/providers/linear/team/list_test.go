package team

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.TeamService double.
type fakeService struct {
	teams []service.Team
	err   error
}

func (f *fakeService) ListTeams(context.Context) ([]service.Team, error) {
	return f.teams, f.err
}

func newListCmdForTest(svc *fakeService, format string) *cobra.Command {
	return newListCmd(cmdtest.NewTestConfig(format),
		func(context.Context) (service.TeamService, error) { return svc, nil })
}

func TestListJSON(t *testing.T) {
	svc := &fakeService{teams: []service.Team{
		{ID: "team_1", Name: "Engineering", Key: "ENG"},
		{ID: "team_2", Name: "Design", Key: "DES"},
	}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"))

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "team_1", first["id"])
	require.Equal(t, "ENG", first["key"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{teams: []service.Team{{ID: "team_1", Name: "Engineering", Key: "ENG"}}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "table"))
	require.Contains(t, out, "NAME")
	require.Contains(t, out, "KEY")
	require.Contains(t, out, "Engineering")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"))
	require.JSONEq(t, `[]`, out)
}
