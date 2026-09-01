package acl

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(&app.Config{Fs: afero.NewMemMapFs()}, fakeNewSvc(&fakeService{}))

	require.Equal(t, "acl", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.Equal(t, []string{"add", "list", "remove"}, names)
}

func TestLeafExamples(t *testing.T) {
	// Every leaf must carry an Example with at least two everything-cli
	// invocations; list must show a --format json call.
	cmd := NewCmd(&app.Config{Fs: afero.NewMemMapFs()}, fakeNewSvc(&fakeService{}))
	for _, leaf := range cmd.Commands() {
		require.NotEmpty(t, leaf.Example, "%s needs an Example", leaf.Name())
		require.GreaterOrEqual(t, strings.Count(leaf.Example, "everything-cli "), 2,
			"%s Example needs at least 2 invocations", leaf.Name())
		require.Contains(t, leaf.Example, "# ", "%s Example needs comments", leaf.Name())
	}
	list := subByName(t, cmd.Commands(), "list")
	require.Contains(t, list.Example, "--format json", "list Example needs a --format json call")
}

func subByName(t *testing.T, subs []*cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, sub := range subs {
		if sub.Name() == name {
			return sub
		}
	}
	t.Fatalf("subcommand %q not found", name)
	return nil
}
