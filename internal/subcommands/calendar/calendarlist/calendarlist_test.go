package calendarlist

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestNewLeavesNames(t *testing.T) {
	leaves := NewLeaves(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))

	var names []string
	for _, leaf := range leaves {
		names = append(names, leaf.Name())
	}
	require.Equal(t, []string{"list", "get", "create", "update", "delete"}, names)
}

func TestLeafExamples(t *testing.T) {
	// Every leaf must carry a flush-left Example with at least two
	// google-cli invocations; reads must show a --format json call.
	leaves := NewLeaves(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))
	for _, leaf := range leaves {
		require.NotEmpty(t, leaf.Example, "%s needs an Example", leaf.Name())
		require.GreaterOrEqual(t, strings.Count(leaf.Example, "google-cli "), 2,
			"%s Example needs at least 2 invocations", leaf.Name())
		require.Contains(t, leaf.Example, "# ", "%s Example needs comments", leaf.Name())
	}
	for _, name := range []string{"list", "get"} {
		leaf := leafByName(t, leaves, name)
		require.Contains(t, leaf.Example, "--format json", "%s Example needs a --format json call", name)
	}
}

func leafByName(t *testing.T, leaves []*cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, leaf := range leaves {
		if leaf.Name() == name {
			return leaf
		}
	}
	t.Fatalf("leaf %q not found", name)
	return nil
}
