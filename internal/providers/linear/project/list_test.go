package project

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

// fakeService is the hermetic service.ProjectService double.
type fakeService struct {
	projects []service.Project
	err      error
}

func (f *fakeService) ListProjects(context.Context) ([]service.Project, error) {
	return f.projects, f.err
}

func newListCmdForTest(svc *fakeService, format string) *cobra.Command {
	return newListCmd(cmdtest.NewTestConfig(format),
		func(context.Context) (service.ProjectService, error) { return svc, nil })
}

func TestListJSON(t *testing.T) {
	svc := &fakeService{projects: []service.Project{
		{ID: "proj_1", Name: "Rewrite", Description: "Rebuild the core", State: "started"},
	}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"))

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one project renders as a single object: %v", out)
	require.Equal(t, "proj_1", m["id"])
	require.Equal(t, "started", m["state"])
	require.Equal(t, "Rebuild the core", m["description"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{projects: []service.Project{{ID: "proj_1", Name: "Rewrite", State: "started"}}}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "table"))
	require.Contains(t, out, "NAME")
	require.Contains(t, out, "STATE")
	require.Contains(t, out, "Rewrite")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newListCmdForTest(svc, "json"))
	require.JSONEq(t, `[]`, out)
}
