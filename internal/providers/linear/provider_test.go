package linear

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/provider"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

func TestProviderRegistersItself(t *testing.T) {
	p, ok := provider.Get("linear")
	require.True(t, ok)
	require.Equal(t, "linear", p.ID())
}

func TestAuthStrategyShape(t *testing.T) {
	s := Provider{}.Auth()
	require.NotNil(t, s)
	require.Equal(t, []string{"auth.api_key"}, s.SecretFields())
}

func TestNewCmdExposesResourceTrees(t *testing.T) {
	cmd := Provider{}.NewCmd(nil)
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"issue", "team", "project", "account"} {
		require.True(t, names[want], "missing subtree %q", want)
	}
}
