package drive

import (
	"bytes"
	"context"
	"testing"
	"time"

	drive "google.golang.org/api/drive/v3"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/file"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
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

// fakeFileSvc is a FileService stub for the wiring tests below: only the
// methods the sharing leaves and list call are implemented; the rest of the
// surfaces are embedded and never invoked.
type fakeFileSvc struct {
	service.FileService
	service.PermissionService
}

func (fakeFileSvc) ListFiles(context.Context, string, int64) ([]*drive.File, error) {
	return nil, nil
}

func (fakeFileSvc) ListPermissions(context.Context, string) ([]*drive.Permission, error) {
	return nil, nil
}

func (fakeFileSvc) GrantPermission(_ context.Context, _ string, p *drive.Permission) (*drive.Permission, error) {
	return p, nil
}

func (fakeFileSvc) DeletePermission(context.Context, string, string) error { return nil }

// TestDialAcceptsDriveFileScope pins the alternatives guard: a minimal
// drive.file-only account (app-created files only) constructs the service for
// the read/write file leaves without re-consent.
func TestDialAcceptsDriveFileScope(t *testing.T) {
	svc, err := dial(context.Background(), newDialConfig(t, "work", []string{auth.ScopeUserEmail, auth.ScopeDriveFile}))
	require.NoError(t, err)
	require.NotNil(t, svc)
}

// TestFileSharingLeavesRequireFullDriveScope pins the sharing guard: the
// read/write leaves run on a drive.file-only account, but the sharing leaves
// (permissions, share, unshare) refuse it — naming the full drive scope and
// the re-consent action — before the dialer is ever called. A full-drive
// account still shares.
func TestFileSharingLeavesRequireFullDriveScope(t *testing.T) {
	driveFileOnly := []string{auth.ScopeUserEmail, auth.ScopeDriveFile}
	fullDrive := []string{auth.ScopeUserEmail, auth.ScopesDrive[0]}

	tests := []struct {
		name       string
		scopes     []string
		args       []string
		wantDialed bool
	}{
		{
			name:       "list runs on a drive.file-only account",
			scopes:     driveFileOnly,
			args:       []string{"list"},
			wantDialed: true,
		},
		{
			name:   "permissions is refused on a drive.file-only account",
			scopes: driveFileOnly,
			args:   []string{"permissions", "file_1"},
		},
		{
			name:   "share is refused on a drive.file-only account",
			scopes: driveFileOnly,
			args:   []string{"share", "file_1", "--role", "reader", "--anyone"},
		},
		{
			name:   "unshare is refused on a drive.file-only account",
			scopes: driveFileOnly,
			args:   []string{"unshare", "file_1", "--permission", "p1"},
		},
		{
			name:       "share runs on a full-drive account",
			scopes:     fullDrive,
			args:       []string{"share", "file_1", "--role", "reader", "--anyone"},
			wantDialed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newDialConfig(t, "work", tc.scopes)
			dialed := false
			cmd := file.NewCmd(cfg, func(context.Context) (service.FileService, error) {
				dialed = true
				return fakeFileSvc{}, nil
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.wantDialed {
				require.NoError(t, err)
				require.True(t, dialed, "leaf must reach the dialer")
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), `account "work"`)
			assert.Contains(t, err.Error(), auth.ScopesDrive[0], "error must name the full drive scope")
			assert.Contains(t, err.Error(), "account add", "error must name the re-consent action")
			assert.False(t, dialed, "guard must fail before the dialer is called")
		})
	}
}
