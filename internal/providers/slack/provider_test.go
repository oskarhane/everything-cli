package slack

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/provider"
)

// TestProviderWiresAccountAndResourceTrees: the core scaffold self-registers
// the provider; provider.go wires the real account subtree plus one parent
// per resource tree. Every stub was replaced by its owning node, so the
// trees are now runnable and their leaf wiring is pinned here. provider.go
// itself is final from here on.
func TestProviderWiresAccountAndResourceTrees(t *testing.T) {
	reg, ok := provider.Get(providerID)
	require.True(t, ok)
	assert.Equal(t, providerID, reg.ID())

	cmd := Provider{}.NewCmd(nil)
	assert.Equal(t, "slack", cmd.Use)

	byName := map[string]*cobra.Command{}
	for _, sub := range cmd.Commands() {
		byName[sub.Name()] = sub
	}

	for _, name := range []string{"account", "search", "channel", "user", "thread", "file"} {
		require.Contains(t, byName, name, "provider.go must wire %q", name)
	}

	// The resource trees are real: search, channel, user, and file are parents
	// with leaves; thread is a runnable leaf.
	for _, name := range []string{"search", "channel", "user", "file"} {
		assert.NotEmpty(t, byName[name].Commands(), "resource tree %q must wire its leaves", name)
	}
	assert.True(t, byName["thread"].Runnable(), "thread must be a runnable leaf")

	// The account subtree is real: add plus the four shared leaves.
	assert.Len(t, byName["account"].Commands(), 5)

	_, root, out := newSlackEnv(t)
	help, err := execute(t, root, out, "slack", "--help")
	require.NoError(t, err)
	assert.Contains(t, help, "Read Slack workspaces via the Slack Web API")
}
