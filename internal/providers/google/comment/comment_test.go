package comment

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"))

	require.Equal(t, "comment", cmd.Name())
	var names []string
	for _, leaf := range cmd.Commands() {
		names = append(names, leaf.Name())
	}
	require.ElementsMatch(t, []string{"list", "add", "reply", "resolve", "reopen", "delete"}, names)
}

// TestLeafExamples enforces the example contract on every leaf: flush-left
// comment-led examples with at least two everything-cli invocations, each
// naming the leaf under comment (the subtree hangs under both docs and
// slides, so either parent's path satisfies it). list is the read leaf, so
// it must show machine-readable output.
func TestLeafExamples(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"))
	leaves := cmd.Commands()
	require.NotEmpty(t, leaves)

	for _, leaf := range leaves {
		t.Run(leaf.Name(), func(t *testing.T) {
			example := leaf.Example
			require.NotEmpty(t, example, "every leaf needs an Example")
			require.True(t, strings.HasPrefix(example, "# "), "Example must be flush-left, starting with a # comment")
			require.GreaterOrEqual(t,
				strings.Count(example, "everything-cli google"),
				2, "Example needs at least two everything-cli invocations")
			require.Contains(t, example, " comment "+leaf.Name()+" ",
				"Example must invoke the leaf under comment")
			if leaf.Name() == "list" {
				require.Contains(t, example, "--format json")
			}
		})
	}
}
