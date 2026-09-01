package drive

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
)

// installedAppCredentials is a minimal valid installed-app credentials file;
// the drive tree only parses it, it never talks to Google's endpoints.
const installedAppCredentials = `{
  "installed": {
    "client_id": "test-client-id",
    "client_secret": "test-client-secret",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost"]
  }
}`

// newDialConfig seeds a hermetic account store on an in-memory FS (never the
// real ~/.config/google-cli) with a valid token plus the given scopes, and
// returns the config dial should run against.
func newDialConfig(t *testing.T, name string, scopes []string) *app.Config {
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

// TestDialRequiresDriveScope pins the scope guard: an account narrowed to
// non-drive scopes must fail with the re-consent guidance before any service
// is built or API call made, instead of surfacing a raw 403 from Google.
func TestDialRequiresDriveScope(t *testing.T) {
	tests := []struct {
		name    string
		scopes  []string
		missing bool
	}{
		{
			name:    "account with the drive scope dials",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesDrive[0]},
			missing: false,
		},
		{
			name:    "account without the drive scope is told to re-consent",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesGmail[0]},
			missing: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := dial(context.Background(), newDialConfig(t, "work", tc.scopes))
			if !tc.missing {
				require.NoError(t, err)
				require.NotNil(t, svc)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), `account "work"`)
			assert.Contains(t, err.Error(), "account add", "error must name the re-consent action")
			assert.Contains(t, err.Error(), auth.ScopesDrive[0], "error must name the missing scope")
		})
	}
}
