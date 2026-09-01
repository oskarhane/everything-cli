package account

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
)

// addNeverPrintsKey asserts the captured key appears in no output format.
func addNeverPrintsKey(t *testing.T, factory StrategyFactory, key string, args ...string) {
	t.Helper()
	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			_, root, out := newAccountEnv(t, factory)
			stdout, err := execute(t, root, out, append(args, "--format", format)...)
			require.NoError(t, err)
			require.NotContains(t, stdout, key)
		})
	}
}

func TestAddCapturesKeyFromFlag(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))

	stdout, err := execute(t, root, out, "account", "add", "work", "--api-key", "test-key-123", "--format", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"name": "work"`)
	require.Contains(t, stdout, `"provider": "linear"`)
	require.NotContains(t, stdout, "test-key-123")
	require.Equal(t, "test-key-123", storedKey(t, cfg, "work"))

	// The first account added becomes the provider default.
	def, err := newStore(t, cfg).DefaultAccountFor(testProviderID)
	require.NoError(t, err)
	require.Equal(t, "work", def)
}

func TestAddCapturesKeyFromEnv(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "test-key-456")
	cfg, root, out := newAccountEnv(t, realStrategy(t))

	_, err := execute(t, root, out, "account", "add", "work")
	require.NoError(t, err)
	require.Equal(t, "test-key-456", storedKey(t, cfg, "work"))
}

func TestAddFlagWinsOverEnv(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "test-key-456")
	cfg, root, out := newAccountEnv(t, realStrategy(t))

	_, err := execute(t, root, out, "account", "add", "work", "--api-key", "test-key-123")
	require.NoError(t, err)
	require.Equal(t, "test-key-123", storedKey(t, cfg, "work"))
}

func TestAddCapturesKeyFromHiddenPrompt(t *testing.T) {
	fake := &fakePromptStrategy{key: "test-key-789"}
	cfg, root, out := newAccountEnv(t, func() auth.Strategy { return fake })

	_, err := execute(t, root, out, "account", "add", "work")
	require.NoError(t, err)
	require.Equal(t, "work", fake.got.Name)
	require.Empty(t, fake.got.APIKey, "prompt path must not see a flag key")
	require.Equal(t, "test-key-789", storedKey(t, cfg, "work"))
}

func TestAddNeverPrintsTheKey(t *testing.T) {
	addNeverPrintsKey(t, realStrategy(t), "test-key-123",
		"account", "add", "work", "--api-key", "test-key-123")
}
