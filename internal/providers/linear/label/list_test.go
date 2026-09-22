package label

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

// fakeService is the hermetic service.LabelService double.
type fakeService struct {
	labels []service.Label
	err    error
}

func (f *fakeService) ListTeamLabels(context.Context, string) ([]service.Label, error) {
	return f.labels, f.err
}

func newListCmdForTest(svc *fakeService, format string) *cobra.Command {
	return newListCmd(cmdtest.NewTestConfig(format),
		func(context.Context) (service.LabelService, error) { return svc, nil })
}

func TestListJSON(t *testing.T) {
	svc := &fakeService{labels: []service.Label{
		{ID: "label_1", Name: "Bug", Color: "#ff0000"},
		{ID: "label_2", Name: "Feature", Color: "#00ff00"},
	}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"), "--team", "team_1")

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "label_1", first["id"])
	require.Equal(t, "Bug", first["name"])
	require.Equal(t, "#ff0000", first["color"])
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, first))
}

func TestListTable(t *testing.T) {
	svc := &fakeService{labels: []service.Label{
		{ID: "label_1", Name: "Bug", Color: "#ff0000"},
	}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "table"), "--team", "team_1")
	require.Contains(t, out, "ID")
	require.Contains(t, out, "NAME")
	require.Contains(t, out, "COLOR")
	require.Contains(t, out, "Bug")
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
