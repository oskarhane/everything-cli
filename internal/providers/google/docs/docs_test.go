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
	// Note: insert rides InsertDocText, added to the committed service as
	// part of this node.
	require.ElementsMatch(t, []string{"get", "append", "insert", "replace", "delete"}, names)
}
