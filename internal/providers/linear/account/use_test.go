package account

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUseSetsDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))
	seedAccount(t, cfg, "work", "test-key-123")
	seedAccount(t, cfg, "personal", "test-key-456")

	stdout, err := execute(t, root, out, "account", "use", "personal")
	require.NoError(t, err)
	require.Contains(t, stdout, "default account set to personal")

	def, err := newStore(t, cfg).DefaultAccountFor(testProviderID)
	require.NoError(t, err)
	require.Equal(t, "personal", def)
}

func TestUseUnknownAccountFails(t *testing.T) {
	_, root, out := newAccountEnv(t, realStrategy(t))

	_, err := execute(t, root, out, "account", "use", "ghost")
	require.Error(t, err)
}
