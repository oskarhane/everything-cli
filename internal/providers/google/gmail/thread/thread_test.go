package thread

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))

	require.Equal(t, "thread", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.ElementsMatch(t, []string{"list", "get"}, names)
}
