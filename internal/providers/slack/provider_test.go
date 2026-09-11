package slack

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/provider"
)

// TestProviderWiresAccountAndResourceStubs: the core scaffold self-registers
// the provider; provider.go wires the real account subtree plus one
// non-runnable stub parent for each future resource tree (search, channel,
// user, thread) so help and drift guards stay inert until the owning node
// replaces the stub file. provider.go itself is final from here on.
func TestProviderWiresAccountAndResourceStubs(t *testing.T) {
	reg, ok := provider.Get(providerID)
	require.True(t, ok)
	assert.Equal(t, providerID, reg.ID())

	cmd := Provider{}.NewCmd(nil)
	assert.Equal(t, "slack", cmd.Use)

	byName := map[string]*cobra.Command{}
	for _, sub := range cmd.Commands() {
		byName[sub.Name()] = sub
	}

	for _, name := range []string{"account", "search", "channel", "user", "thread"} {
		require.Contains(t, byName, name, "provider.go must wire %q", name)
	}

	for _, name := range []string{"search", "channel", "user", "thread"} {
		sub := byName[name]
		assert.Nil(t, sub.Run, "stub %q must have no Run", name)
		assert.Nil(t, sub.RunE, "stub %q must have no RunE", name)
		assert.False(t, sub.Runnable(), "stub %q must not be runnable", name)
		assert.Empty(t, sub.Commands(), "stub %q must have no children", name)
	}

	// The account subtree is real: add plus the four shared leaves.
	assert.Len(t, byName["account"].Commands(), 5)

	_, root, out := newSlackEnv(t)
	help, err := execute(t, root, out, "slack", "--help")
	require.NoError(t, err)
	assert.Contains(t, help, "Read Slack workspaces via the Slack Web API")
}
