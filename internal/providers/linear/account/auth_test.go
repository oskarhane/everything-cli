package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// seedOAuthAccount persists an OAuth-shaped linear account (viewer email,
// narrowed scopes, stale token, client credentials payload) directly in the
// store — the state `linear account auth` re-authorizes.
func seedOAuthAccount(t *testing.T, cfg *app.Config, name string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
	})
	require.NoError(t, err)
	require.NoError(t, newStore(t, cfg).Save(&config.Account{
		Name:     name,
		Provider: testProviderID,
		Email:    "viewer@example.com",
		Scopes:   []string{"read"},
		Token: &oauth2.Token{
			AccessToken:  "stale-access",
			RefreshToken: "stale-refresh",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(-time.Hour),
		},
		Auth: payload,
	}))
}

// fakeReauthStrategy stands in for the composite strategy on the re-auth
// path: it records the account and scopes it was handed, optionally fails,
// and persists a fresh token through the real store exactly like the OAuth
// strategy does — under the account's own name, payload preserved.
type fakeReauthStrategy struct {
	err       error
	invoked   bool
	gotName   string
	gotScopes []string
	token     string
}

func (f *fakeReauthStrategy) Add(context.Context, afero.Fs, *config.Store, auth.AddOptions) (*config.Account, error) {
	return nil, errors.New("fakeReauthStrategy has no add")
}

func (f *fakeReauthStrategy) Client(context.Context, *config.Account) (*http.Client, error) {
	return nil, errors.New("fakeReauthStrategy has no client")
}

func (f *fakeReauthStrategy) Reauth(_ context.Context, _ afero.Fs, store *config.Store, acct *config.Account, opts auth.ReauthOptions) (*config.Account, error) {
	f.invoked = true
	f.gotName = acct.Name
	f.gotScopes = opts.Scopes
	if f.err != nil {
		return nil, f.err
	}
	updated := *acct
	updated.Token = &oauth2.Token{
		AccessToken: f.token,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
	if err := store.Save(&updated); err != nil {
		return nil, err
	}
	return store.GetProvider(testProviderID, updated.Name)
}

func TestAuthReauthorizesOAuthAccountInPlace(t *testing.T) {
	fake := &fakeReauthStrategy{token: "fresh-access"}
	cfg, root, out := newAccountEnv(t, func(*config.Store) auth.Strategy { return fake })
	seedOAuthAccount(t, cfg, "work")

	stdout, err := execute(t, root, out, "account", "auth", "work", "--format", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"name": "work"`)
	require.Contains(t, stdout, `"provider": "linear"`)
	require.NotContains(t, stdout, "fresh-access", "the token must never print")
	require.NotContains(t, stdout, "test-client-secret", "the client secret must never print")

	// Re-auth was handed the stored account; absent --scopes means no
	// override (nil), and the Reauther keeps the account's granted scopes
	// — that cascade is pinned by the strategy-level reauth tests.
	require.True(t, fake.invoked)
	require.Equal(t, "work", fake.gotName)
	require.Empty(t, fake.gotScopes, "absent --scopes sends no override")

	// The account was updated in place: same name, fresh token, payload
	// preserved.
	persisted, err := newStore(t, cfg).GetProvider(testProviderID, "work")
	require.NoError(t, err)
	require.NotNil(t, persisted.Token)
	require.Equal(t, "fresh-access", persisted.Token.AccessToken)
	var payload map[string]string
	require.NoError(t, json.Unmarshal(persisted.Auth, &payload))
	require.Equal(t, "test-client-id", payload["client_id"])
	require.Equal(t, "test-client-secret", payload["client_secret"])
}

func TestAuthScopesFlagOverridesAccountScopes(t *testing.T) {
	fake := &fakeReauthStrategy{token: "fresh-access"}
	cfg, root, out := newAccountEnv(t, func(*config.Store) auth.Strategy { return fake })
	seedOAuthAccount(t, cfg, "work")

	_, err := execute(t, root, out, "account", "auth", "work", "--scopes", "read, write ,")
	require.NoError(t, err)
	require.Equal(t, []string{"read", "write"}, fake.gotScopes)
}

func TestAuthUnknownAccountGuidesToAdd(t *testing.T) {
	fake := &fakeReauthStrategy{token: "fresh-access"}
	_, root, out := newAccountEnv(t, func(*config.Store) auth.Strategy { return fake })

	_, err := execute(t, root, out, "account", "auth", "ghost")
	require.ErrorContains(t, err, `no linear account "ghost"`)
	require.ErrorContains(t, err, "linear account add")
	require.False(t, fake.invoked, "re-auth must not run for an unknown account")
}

// TestAuthRejectsStrategiesWithoutReauth pins the belt: the leaf
// type-asserts the Reauther capability rather than assuming it, so a
// strategy with no interactive flow is rejected with a plain error.
func TestAuthRejectsStrategiesWithoutReauth(t *testing.T) {
	cfg, root, out := newAccountEnv(t, realStrategy(t))
	seedAccount(t, cfg, "work", "test-key-123")

	_, err := execute(t, root, out, "account", "auth", "work")
	require.ErrorContains(t, err, "linear accounts do not support re-auth")
}

func TestAuthWrapsReauthFailure(t *testing.T) {
	fake := &fakeReauthStrategy{err: errors.New("flow canceled")}
	cfg, root, out := newAccountEnv(t, func(*config.Store) auth.Strategy { return fake })
	seedOAuthAccount(t, cfg, "work")

	_, err := execute(t, root, out, "account", "auth", "work")
	require.ErrorContains(t, err, `re-authorizing account "work"`)
	require.ErrorContains(t, err, "flow canceled")
}

// TestParseScopes pins the --scopes parsing: comma-split, trimmed, blanks
// dropped, empty (or blank-only) input yielding nil.
func TestParseScopes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "blank only", in: " , , ", want: []string{}},
		{name: "trims and drops blanks", in: " read,write , ,issues:create", want: []string{"read", "write", "issues:create"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, parseScopes(tc.in))
		})
	}
}
