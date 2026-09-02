package account

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/auth"
)

// TestAddRunsFlowAndSavesAccount: add resolves and parses the credentials
// file, runs the OAuth flow with the default scope set, saves the account,
// and prints its name and email.
func TestAddRunsFlowAndSavesAccount(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")

	var gotCreds auth.ClientCredentials
	var gotScopes []string
	stubAddStrategy(t, func(creds auth.ClientCredentials, scopes []string) (*oauth2.Token, string, error) {
		gotCreds, gotScopes = creds, scopes
		return testToken("work"), "user@example.com", nil
	})

	outStr, err := execute(t, root, out, "account", "add", "work")
	require.NoError(t, err)

	assert.Equal(t, "test-client-id", gotCreds.ID,
		"the client credentials from the auto-resolved file must reach the flow")
	assert.Equal(t, auth.GoogleOAuth.DefaultScopes, gotScopes, "no --scopes flag means the default scope set")
	assert.Contains(t, outStr, "work")
	assert.Contains(t, outStr, "user@example.com")

	saved, err := newStore(t, cfg).Get("work")
	require.NoError(t, err)
	assert.Equal(t, "google", saved.Provider, "the account persists under the google provider")
	assert.Equal(t, "user@example.com", saved.Email)
	assert.Equal(t, auth.GoogleOAuth.DefaultScopes, saved.Scopes)
	assert.Equal(t, "secret-access-work", saved.Token.AccessToken, "the token is cached on disk")
}

// TestDefaultScopes pins the exact default scope set the profile falls back
// to: full Gmail, Calendar, Drive, Docs, Sheets and Slides access, plus
// userinfo.email.
func TestDefaultScopes(t *testing.T) {
	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.send",
		"https://www.googleapis.com/auth/gmail.compose",
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/presentations",
		"https://www.googleapis.com/auth/userinfo.email",
	}, auth.GoogleOAuth.DefaultScopes)
}

// TestAddScopesFlag: --scopes overrides the default set, splitting on commas
// and trimming blanks.
func TestAddScopesFlag(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")

	stubAddStrategy(t, func(auth.ClientCredentials, []string) (*oauth2.Token, string, error) {
		return testToken("work"), "user@example.com", nil
	})

	_, err := execute(t, root, out, "account", "add", "work",
		"--scopes", "https://example.com/a, https://example.com/b,,https://example.com/c")
	require.NoError(t, err)

	saved, err := newStore(t, cfg).Get("work")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
	}, saved.Scopes)
}

// TestAddCredentialsFlag: --credentials picks an explicit credentials file
// over the config-dir default.
func TestAddCredentialsFlag(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")
	writeCredentialsWithID(t, cfg, "/elsewhere/credentials.json", "flag-client-id")

	var gotCreds auth.ClientCredentials
	stubAddStrategy(t, func(creds auth.ClientCredentials, _ []string) (*oauth2.Token, string, error) {
		gotCreds = creds
		return testToken("work"), "user@example.com", nil
	})

	_, err := execute(t, root, out, "account", "add", "work",
		"--credentials", "/elsewhere/credentials.json")
	require.NoError(t, err)
	assert.Equal(t, "flag-client-id", gotCreds.ID,
		"the flag's credentials file must be the one parsed")
}

// TestAddWithoutCredentialsErrors: no credentials file anywhere yields the
// resolution error and no account is saved.
func TestAddWithoutCredentialsErrors(t *testing.T) {
	cfg, root, out := newAccountEnv(t)

	stubAddStrategy(t, func(auth.ClientCredentials, []string) (*oauth2.Token, string, error) {
		t.Fatal("the flow must not run without credentials")
		return nil, "", nil
	})

	_, err := execute(t, root, out, "account", "add", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth credentials file found")

	accounts, err := newStore(t, cfg).List()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

// TestAddFlowErrorPropagates: a failed authorization surfaces the flow error
// and saves nothing.
func TestAddFlowErrorPropagates(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")

	stubAddStrategy(t, func(auth.ClientCredentials, []string) (*oauth2.Token, string, error) {
		return nil, "", errors.New("flow blew up")
	})

	_, err := execute(t, root, out, "account", "add", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adding account \"work\"")
	assert.Contains(t, err.Error(), "flow blew up")

	accounts, err := newStore(t, cfg).List()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

// TestAddDedupesByEmail: adding an account whose email already exists saves
// under the original account's name and prints that canonical name.
func TestAddDedupesByEmail(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	writeCredentials(t, cfg, "/config/credentials.json")
	seedAccount(t, cfg, "work", "user@example.com")

	stubAddStrategy(t, func(auth.ClientCredentials, []string) (*oauth2.Token, string, error) {
		return testToken("alt"), "user@example.com", nil
	})

	outStr, err := execute(t, root, out, "account", "add", "alt")
	require.NoError(t, err)
	assert.Contains(t, outStr, "work", "the canonical name must be printed")
	assert.NotContains(t, outStr, "alt")

	accounts, err := newStore(t, cfg).List()
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "work", accounts[0].Name)
}
