package state

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.StateService double.
type fakeService struct {
	states []service.State
	err    error
}

func (f *fakeService) ListStates(context.Context, string) ([]service.State, error) {
	return f.states, f.err
}

func newListCmdForTest(svc *fakeService, format string) *cobra.Command {
	return newListCmd(cmdtest.NewTestConfig(format),
		func(context.Context) (service.StateService, error) { return svc, nil })
}

func TestListJSON(t *testing.T) {
	svc := &fakeService{states: []service.State{
		{ID: "state_1", Name: "Backlog", Type: "backlog", Position: 0},
		{ID: "state_2", Name: "In Progress", Type: "started", Position: 2.5},
	}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"), "--team", "team_1")

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "state_1", first["id"])
	require.Equal(t, "Backlog", first["name"])
	require.Equal(t, "backlog", first["type"])
	require.Equal(t, float64(0), first["position"])
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, first))
}

func TestListTable(t *testing.T) {
	svc := &fakeService{states: []service.State{
		{ID: "state_1", Name: "Backlog", Type: "backlog", Position: 0},
	}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "table"), "--team", "team_1")
	require.Contains(t, out, "ID")
	require.Contains(t, out, "NAME")
	require.Contains(t, out, "TYPE")
	require.Contains(t, out, "POSITION")
	require.Contains(t, out, "Backlog")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"), "--team", "team_1")
	require.JSONEq(t, `[]`, out)
}

func TestListMissingTeam(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newListCmdForTest(svc, "json"))
	require.Error(t, err)
}

func TestListServiceError(t *testing.T) {
	svc := &fakeService{err: errors.New("boom")}
	_, err := cmdtest.RunCmdErr(t, newListCmdForTest(svc, "json"), "--team", "team_1")
	require.ErrorContains(t, err, "boom")
}
