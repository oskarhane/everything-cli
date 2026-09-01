package drive

import (
	"bytes"
	"context"
	"testing"

	drive "google.golang.org/api/drive/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/file"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

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
			svc, err := dial(context.Background(), cmdtest.NewDialConfig(t, "work", tc.scopes))
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
	svc, err := dial(context.Background(), cmdtest.NewDialConfig(t, "work", []string{auth.ScopeUserEmail, auth.ScopeDriveFile}))
	require.NoError(t, err)
	require.NotNil(t, svc)
}

// TestFileSharingLeavesRequireFullDriveScope pins the sharing guard: the
// read/write leaves run on a drive.file-only account, but the sharing leaves
// (permissions, share, unshare) refuse it — naming the full drive scope and
// the re-consent action — before the dialer is ever called. The full-drive
// success path builds the service inside newSharingSvc and is not covered by
// any test (it would dial Google live); only the refusal paths are pinned
// here, and the leaf-level grant behavior is faked in file/share_test.go.
func TestFileSharingLeavesRequireFullDriveScope(t *testing.T) {
	driveFileOnly := []string{auth.ScopeUserEmail, auth.ScopeDriveFile}

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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := cmdtest.NewDialConfig(t, "work", tc.scopes)
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
