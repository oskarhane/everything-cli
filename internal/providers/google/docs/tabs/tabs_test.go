package tabs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestTabsLeaves pins the subtree's shape: exactly the four leaves — list,
// create, delete, rename — and none of them registers a --format of its own
// (--format is the root's persistent flag; list only renders from it).
func TestTabsLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig(""), fakeNewSvc(&fakeDocService{}))

	leaves := map[string]bool{}
	for _, sub := range cmd.Commands() {
		leaves[sub.Name()] = true
		require.Nil(t, sub.Flags().Lookup("format"), "%s must not register --format", sub.Name())
	}
	require.Len(t, cmd.Commands(), 4)
	require.True(t, leaves["list"] && leaves["create"] && leaves["delete"] && leaves["rename"], "leaf set = %v", leaves)
}

// TestTabsLeavesCarryExamples pins the whole-tree examples gate locally:
// every leaf carries a flush-left Example with at least two invocations.
func TestTabsLeavesCarryExamples(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig(""), fakeNewSvc(&fakeDocService{}))

	for _, sub := range cmd.Commands() {
		require.NotEmpty(t, sub.Example, "%s must document invocations", sub.Name())
		require.Regexp(t, `(?m)^everything-cli `, sub.Example,
			"%s needs runnable example lines", sub.Name())
		require.Regexp(t, `(?m)\neverything-cli `, sub.Example,
			"%s needs at least two example invocations", sub.Name())
	}
}
