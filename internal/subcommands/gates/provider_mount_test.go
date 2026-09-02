package gates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderLeaves_AreReachable pins that the mounted whole tree actually
// contains linear and granola leaves, so the whole-tree gates
// (TestAllLeafCommands_HaveExamples, TestInputIdentifiers_AreKebabCase)
// provably cover them — a mount regression that drops a provider subtree
// fails here by name instead of silently shrinking the walk.
func TestProviderLeaves_AreReachable(t *testing.T) {
	root, _, _ := mountAndCheck(t)
	for _, path := range [][]string{
		{"linear", "issue", "list"},
		{"granola", "note", "list"},
		{"granola", "note", "get"},
	} {
		cmd, _, err := root.Find(path)
		require.NoError(t, err, "whole tree must contain %v", path)
		require.NotNil(t, cmd)
		assert.True(t, isRunnableLeaf(cmd),
			"%s should be a runnable leaf", cmd.CommandPath())
	}
}
