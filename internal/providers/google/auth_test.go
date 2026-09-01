package google

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/config"
)

// newTestStore returns a hermetic accounts store on an in-memory FS: tests
// must never read or write the real ~/.config/google-cli.
func newTestStore(t *testing.T) *config.Store {
	t.Helper()
	store, err := config.NewStore(afero.NewMemMapFs(), "/config")
	require.NoError(t, err)
	return store
}

// TestStrategyIsAnOAuthStrategy: Google's strategy composes the shared
// OAuth machinery with the pinned GoogleOAuth profile.
func TestStrategyIsAnOAuthStrategy(t *testing.T) {
	s := NewStrategy(afero.NewMemMapFs(), newTestStore(t), "/config/credentials.json")
	require.NotNil(t, s.OAuthStrategy, "the google strategy must embed the shared OAuth strategy")
}

// TestSecretFields: Google's secret fields name both OAuth token values;
// each is registered for redaction at the mint/read point.
func TestSecretFields(t *testing.T) {
	s := NewStrategy(afero.NewMemMapFs(), newTestStore(t), "")
	fields := s.SecretFields()
	var joined string
	for _, f := range fields {
		joined += f + "\n"
	}
	assert.Contains(t, joined, "access_token")
	assert.Contains(t, joined, "refresh_token")
	assert.Equal(t, fields, s.SecretFields(),
		"SecretFields must return a fresh copy a caller cannot mutate")
	fields[0] = "mutated"
	assert.NotEqual(t, fields, s.SecretFields())
}

// TestClientRejectsNilAccount: Client fails closed on a nil account.
func TestClientRejectsNilAccount(t *testing.T) {
	s := NewStrategy(afero.NewMemMapFs(), newTestStore(t), "")
	_, err := s.Client(context.Background(), nil)
	require.Error(t, err)
}
