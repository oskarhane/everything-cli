package drive

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestNewCmdRegistersFile(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"))

	require.Equal(t, "drive", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.Equal(t, []string{"file"}, names)
}
