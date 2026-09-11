package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/provider"
)

// TestProviderRegistersBareParent: the core scaffold self-registers the
// provider and its command tree is a bare `slack` parent — the account and
// resource subtrees are wired by their own nodes.
func TestProviderRegistersBareParent(t *testing.T) {
	reg, ok := provider.Get(providerID)
	require.True(t, ok)
	assert.Equal(t, providerID, reg.ID())

	cmd := Provider{}.NewCmd(nil)
	assert.Equal(t, "slack", cmd.Use)
	assert.Empty(t, cmd.Commands())

	_, root, out := newSlackEnv(t)
	help, err := execute(t, root, out, "slack", "--help")
	require.NoError(t, err)
	assert.Contains(t, help, "Read Slack workspaces via the Slack Web API")
}
