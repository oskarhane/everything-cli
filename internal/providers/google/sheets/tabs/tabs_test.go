package tabs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestTabsLeaves pins the subtree's shape: exactly the three write leaves —
// no list leaf, listing stays with `sheets get` — and none of them
// registers a --format of its own (they render a fixed confirmation echo;
// --format is the root's persistent flag).
func TestTabsLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig(""), fakeNewSvc(&fakeTabService{}))

	leaves := map[string]bool{}
	for _, sub := range cmd.Commands() {
		leaves[sub.Name()] = true
		require.Nil(t, sub.Flags().Lookup("format"), "%s must not register --format", sub.Name())
	}
	require.Len(t, cmd.Commands(), 3)
	require.True(t, leaves["create"] && leaves["delete"] && leaves["rename"], "leaf set = %v", leaves)
	require.False(t, leaves["list"], "listing stays with `sheets get` — tabs adds no list leaf")
}
