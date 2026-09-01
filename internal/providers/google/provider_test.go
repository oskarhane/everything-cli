package google

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/provider"
	"github.com/spf13/afero"
)

// TestProviderRegistered: init self-registration makes the provider
// discoverable through the registry under "google".
func TestProviderRegistered(t *testing.T) {
	p, ok := provider.Get("google")
	require.True(t, ok, "google provider must be registered")
	assert.Equal(t, "google", p.ID())
}

// TestNewCmdMountsResourceTrees: the google parent carries every Google
// resource subtree plus the provider-scoped account subtree.
func TestNewCmdMountsResourceTrees(t *testing.T) {
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	cmd := Provider{}.NewCmd(cfg)

	assert.Equal(t, "google", cmd.Use)
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, want := range []string{
		"account", "gmail", "calendar", "drive", "docs", "sheets", "slides", "youtube",
	} {
		assert.True(t, found[want], "google command missing subtree %q", want)
	}
}

// TestCredentialsIsGooglePersistentFlag: --credentials is Google-specific,
// so it is a persistent flag on the google command, inherited by every
// Google leaf, and binds to cfg.Credentials.
func TestCredentialsIsGooglePersistentFlag(t *testing.T) {
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	cmd := Provider{}.NewCmd(cfg)

	flag := cmd.PersistentFlags().Lookup("credentials")
	require.NotNil(t, flag, "google command must carry a persistent --credentials flag")

	require.NoError(t, cmd.ParseFlags([]string{"--credentials", "creds.json"}))
	assert.Equal(t, "creds.json", cfg.Credentials)
}
