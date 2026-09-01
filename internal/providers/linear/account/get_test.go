package account

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetShowsMetadataOnly(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))
	seedAccount(t, cfg, "work", "test-key-123")

	for _, format := range []string{"json", "table", "toon"} {
		stdout, err := execute(t, root, out, "account", "get", "work", "--format", format)
		require.NoError(t, err)
		require.Contains(t, stdout, "work")
		require.NotContains(t, stdout, "test-key-123", "format %s leaked the API key", format)
	}
}

func TestGetUnknownAccountFails(t *testing.T) {
	_, root, out := newAccountEnv(t, realStrategy(t))

	_, err := execute(t, root, out, "account", "get", "ghost")
	require.Error(t, err)
}
