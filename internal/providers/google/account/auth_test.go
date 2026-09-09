package account

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/config"
)

// TestAuthKeepsAccountScopesAndRotatesToken: auth re-runs the flow for an
// existing account, hands the stored account through with no scope override
// when --scopes is absent (scope preservation itself is pinned by the
// strategy-level tests in internal/auth), replaces the cached token in
// place, and prints the account's name and email.
func TestAuthKeepsAccountScopesAndRotatesToken(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")
	seedAccount(t, cfg, "work", "user@example.com")

	var gotAcct *config.Account
	var gotScopes []string
	stubAuthStrategy(t, func(acct *config.Account, scopes []string) (*oauth2.Token, string, error) {
		gotAcct, gotScopes = acct, scopes
		return testToken("rotated"), "user@example.com", nil
	})

	outStr, err := execute(t, root, out, "account", "auth", "work")
	require.NoError(t, err)

	require.NotNil(t, gotAcct)
	assert.Equal(t, "work", gotAcct.Name)
	assert.Equal(t, "user@example.com", gotAcct.Email)
	assert.Empty(t, gotScopes, "absent --scopes sends no override; keeping the granted scopes is the strategy's job")

	assert.Contains(t, outStr, "work")
	assert.Contains(t, outStr, "user@example.com")

	saved, err := newStore(t, cfg).Get("work")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", saved.Email, "the account is updated in place, not duplicated")
	assert.Equal(t, "secret-access-rotated", saved.Token.AccessToken, "the new token replaces the cached one")
}

// TestAuthScopesFlagOverrides: --scopes is parsed (comma-split, trimmed,
// blanks dropped) and passed to the strategy as the raw override.
func TestAuthScopesFlagOverrides(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")
	seedAccount(t, cfg, "work", "user@example.com")

	var gotScopes []string
	stubAuthStrategy(t, func(_ *config.Account, scopes []string) (*oauth2.Token, string, error) {
		gotScopes = scopes
		return testToken("rotated"), "user@example.com", nil
	})

	_, err := execute(t, root, out, "account", "auth", "work",
		"--scopes", "https://example.com/a, https://example.com/b,,https://example.com/c")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
	}, gotScopes)
}

// TestAuthUnknownAccountErrors: a name with no stored account yields a
// guidance error pointing at account add, and no flow runs.
func TestAuthUnknownAccountErrors(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")

	stubAuthStrategy(t, func(*config.Account, []string) (*oauth2.Token, string, error) {
		t.Fatal("the flow must not run for an unknown account")
		return nil, "", nil
	})

	_, err := execute(t, root, out, "account", "auth", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no google account \"ghost\"")
	assert.Contains(t, err.Error(), "account add")
}

// TestAuthFlowErrorPropagates: a failed re-authorization surfaces the flow
// error, wrapped like add's, and the stored token is untouched.
func TestAuthFlowErrorPropagates(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")
	seedAccount(t, cfg, "work", "user@example.com")

	stubAuthStrategy(t, func(*config.Account, []string) (*oauth2.Token, string, error) {
		return nil, "", errors.New("flow blew up")
	})

	_, err := execute(t, root, out, "account", "auth", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-authorizing account \"work\"")
	assert.Contains(t, err.Error(), "flow blew up")

	saved, err := newStore(t, cfg).Get("work")
	require.NoError(t, err)
	assert.Equal(t, "secret-access-work", saved.Token.AccessToken, "the stored token is unchanged")
}

// TestAuthWithoutCredentialsErrors: no credentials file anywhere yields the
// resolution error before any flow runs, leaving the stored token intact.
func TestAuthWithoutCredentialsErrors(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "work", "user@example.com")

	stubAuthStrategy(t, func(*config.Account, []string) (*oauth2.Token, string, error) {
		t.Fatal("the flow must not run without credentials")
		return nil, "", nil
	})

	_, err := execute(t, root, out, "account", "auth", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth credentials file found")

	saved, err := newStore(t, cfg).Get("work")
	require.NoError(t, err)
	assert.Equal(t, "secret-access-work", saved.Token.AccessToken, "the stored token is unchanged")
}

// TestAuthNeverLeaksTokenSecrets: neither the old nor the new token's secret
// values may reach any output format — auth prints identity only.
func TestAuthNeverLeaksTokenSecrets(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")
	seedAccount(t, cfg, "work", "user@example.com")

	stubAuthStrategy(t, func(*config.Account, []string) (*oauth2.Token, string, error) {
		return testToken("rotated"), "user@example.com", nil
	})

	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			outStr, err := execute(t, root, out, "account", "auth", "work", "--format", format)
			require.NoError(t, err)
			assert.Contains(t, outStr, "user@example.com")
			assert.NotContains(t, outStr, "secret-access-", "access tokens must never be printed")
			assert.NotContains(t, outStr, "secret-refresh-", "refresh tokens must never be printed")
		})
	}
}
