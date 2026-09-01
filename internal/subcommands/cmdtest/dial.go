package cmdtest

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
)

// installedAppCredentials is a minimal valid installed-app credentials file;
// the dial-config tests only parse it, they never talk to Google's endpoints.
const installedAppCredentials = `{
  "installed": {
    "client_id": "test-client-id",
    "client_secret": "test-client-secret",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost"]
  }
}`

// NewDialConfig seeds a hermetic account store on an in-memory FS (never the
// real ~/.config/google-cli) with a valid token plus the given scopes, and
// returns the config dial should run against.
func NewDialConfig(t *testing.T, name string, scopes []string) *app.Config {
	t.Helper()
	fs := afero.NewMemMapFs()
	store, err := config.NewStore(fs, "")
	require.NoError(t, err)
	acct := &config.Account{
		Name:   name,
		Email:  name + "@example.com",
		Scopes: scopes,
		Token: &oauth2.Token{
			AccessToken:  "access-" + name,
			RefreshToken: "refresh-" + name,
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		},
	}
	require.NoError(t, store.Save(acct))
	require.NoError(t, store.SetDefaultAccount(name))
	path := "/config/credentials.json"
	require.NoError(t, afero.WriteFile(fs, path, []byte(installedAppCredentials), 0o600))
	return &app.Config{Fs: fs, Credentials: path}
}
