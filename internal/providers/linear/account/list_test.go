package account

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListMarksDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))
	seedAccount(t, cfg, "work", "test-key-123")
	seedAccount(t, cfg, "personal", "test-key-456")

	stdout, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"name": "work"`)
	require.Contains(t, stdout, `"default": true`)
	// Keys never reach any output format.
	require.NotContains(t, stdout, "test-key-123")
	require.NotContains(t, stdout, "test-key-456")
}

func TestListEmpty(t *testing.T) {
	_, root, out := newAccountEnv(t, realStrategy(t))

	stdout, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)
	require.JSONEq(t, `[]`, stdout)
}
