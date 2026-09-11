package docs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"))

	require.Equal(t, "docs", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	// Note: insert rides the shared InsertDocText service method; comment is
	// the shared Drive-API comment subtree, attached under both docs and
	// slides; tabs is the Docs-API tab subtree.
	require.ElementsMatch(t, []string{"get", "append", "insert", "replace", "delete", "comment", "tabs"}, names)
}
