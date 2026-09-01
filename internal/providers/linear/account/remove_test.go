package account

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveRequiresForce(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))
	seedAccount(t, cfg, "work", "test-key-123")

	_, err := execute(t, root, out, "account", "remove", "work")
	require.ErrorContains(t, err, "--force")
	// The account survived.
	require.Equal(t, "test-key-123", storedKey(t, cfg, "work"))
}

func TestRemoveDeletesAndPromotesDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))
	seedAccount(t, cfg, "work", "test-key-123")
	seedAccount(t, cfg, "personal", "test-key-456")

	stdout, err := execute(t, root, out, "account", "remove", "work", "--force")
	require.NoError(t, err)
	require.Contains(t, stdout, "removed account work")

	accounts, err := newStore(t, cfg).ListProvider(testProviderID)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	// Removing the default promoted the remaining account.
	def, err := newStore(t, cfg).DefaultAccountFor(testProviderID)
	require.NoError(t, err)
	require.Equal(t, "personal", def)
}
