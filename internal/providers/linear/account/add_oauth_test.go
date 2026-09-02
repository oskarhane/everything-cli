package account

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// recordingStrategy captures the AddOptions account add hands the
// strategy, so the OAuth flag plumbing is asserted without a flow.
type recordingStrategy struct {
	got auth.AddOptions
}

func (r *recordingStrategy) Add(_ context.Context, _ afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
	r.got = opts
	acct := &config.Account{Name: opts.Name, Provider: testProviderID}
	if err := store.Save(acct); err != nil {
		return nil, err
	}
	return store.GetProvider(testProviderID, acct.Name)
}

func (r *recordingStrategy) Client(context.Context, *config.Account) (*http.Client, error) {
	return nil, errors.New("recordingStrategy has no client")
}

func TestAddOAuthPassesOAuthOptions(t *testing.T) {
	rec := &recordingStrategy{}
	_, root, out := newAccountEnv(t, func() auth.Strategy { return rec })

	stdout, err := execute(t, root, out, "account", "add", "work",
		"--oauth", "--client-id", "cid-1", "--client-secret", "csecret-1", "--format", "json")
	require.NoError(t, err)
	require.True(t, rec.got.UseOAuth)
	require.Equal(t, "cid-1", rec.got.ClientID)
	require.Equal(t, "csecret-1", rec.got.ClientSecret)
	require.Contains(t, stdout, `"provider": "linear"`)
	require.NotContains(t, stdout, "csecret-1", "the client secret must never print")
}

func TestAddWithoutOAuthKeepsAPIKeyDefault(t *testing.T) {
	rec := &recordingStrategy{}
	_, root, out := newAccountEnv(t, func() auth.Strategy { return rec })

	_, err := execute(t, root, out, "account", "add", "work", "--api-key", "test-key-123")
	require.NoError(t, err)
	require.False(t, rec.got.UseOAuth, "the API-key path stays the default")
	require.Equal(t, "test-key-123", rec.got.APIKey)
	require.Empty(t, rec.got.ClientID)
}

func TestAddAPIKeyAndOAuthAreMutuallyExclusive(t *testing.T) {
	rec := &recordingStrategy{}
	_, root, out := newAccountEnv(t, func() auth.Strategy { return rec })

	_, err := execute(t, root, out, "account", "add", "work", "--api-key", "k", "--oauth")
	require.Error(t, err)
}
