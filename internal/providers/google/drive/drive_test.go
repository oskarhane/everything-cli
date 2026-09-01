package drive

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain pins the output seams so the host's harness env can't flip
// expectations (same pattern as the other subcommand packages).
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

func TestNewCmdRegistersFile(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"))

	require.Equal(t, "drive", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.Equal(t, []string{"file"}, names)
}
