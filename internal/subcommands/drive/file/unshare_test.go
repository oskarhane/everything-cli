package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestUnshareByEmail pins the email path: exactly the one matching permission
// is deleted, and the confirmation names the email.
func TestUnshareByEmail(t *testing.T) {
	svc := &fakeService{perms: seedPermissions()}
	out := cmdtest.RunCmd(t, newLeafCmd(newUnshareCmd, svc, "json"),
		"file_1", "--email", "alice@example.com")

	require.Equal(t, "file_1", svc.listedFileID)
	require.Equal(t, "file_1", svc.deletedFileID)
	require.Equal(t, "perm_1", svc.deletedPermID)
	require.Contains(t, out, "Revoked alice@example.com on file_1")
}

// TestUnshareEmailCaseInsensitive: matching is case-insensitive on the email.
func TestUnshareEmailCaseInsensitive(t *testing.T) {
	svc := &fakeService{perms: seedPermissions()}
	cmdtest.RunCmd(t, newLeafCmd(newUnshareCmd, svc, "json"), "file_1", "--email", "ALICE@EXAMPLE.COM")

	require.Equal(t, "perm_1", svc.deletedPermID)
}

// TestUnshareByPermissionID: the --permission path deletes without listing.
func TestUnshareByPermissionID(t *testing.T) {
	svc := &fakeService{perms: seedPermissions()}
	out := cmdtest.RunCmd(t, newLeafCmd(newUnshareCmd, svc, "json"), "file_1", "--permission", "anyoneWithLink")

	require.Empty(t, svc.listedFileID, "no list call expected when --permission is given")
	require.Equal(t, "anyoneWithLink", svc.deletedPermID)
	require.Contains(t, out, "Revoked anyoneWithLink on file_1")
}

func TestUnshareNoMatch(t *testing.T) {
	svc := &fakeService{perms: seedPermissions()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUnshareCmd, svc, "json"), "file_1", "--email", "ghost@example.com")

	require.Contains(t, err.Error(),
		`no permission for ghost@example.com on file file_1: run "google-cli drive file permissions file_1" to find the permission id`)
	require.Empty(t, svc.deletedPermID, "no revoke may happen on a zero match")
}

// TestUnshareAmbiguousEmail: user and group sharing the same address refuse
// to resolve; the error names both permission ids and suggests --permission.
func TestUnshareAmbiguousEmail(t *testing.T) {
	perms := seedPermissions()
	perms = append(perms,
		&drive.Permission{Id: "perm_user_3", Type: "user", Role: "reader", EmailAddress: "dup@example.com"},
		&drive.Permission{Id: "perm_group_4", Type: "group", Role: "reader", EmailAddress: "DUP@example.com"},
	)
	svc := &fakeService{perms: perms}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUnshareCmd, svc, "json"), "file_1", "--email", "dup@example.com")

	require.Contains(t, err.Error(), "multiple permissions for dup@example.com on file file_1")
	require.Contains(t, err.Error(), "perm_user_3, perm_group_4")
	require.Contains(t, err.Error(), "--permission")
	require.Empty(t, svc.deletedPermID)
}

func TestUnshareTargetValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no flag", []string{"file_1"}, "exactly one of --permission or --email is required"},
		{"both", []string{"file_1", "--permission", "perm_1", "--email", "alice@example.com"},
			"exactly one of --permission or --email is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{perms: seedPermissions()}
			_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUnshareCmd, svc, "json"), tt.args...)

			require.Contains(t, err.Error(), tt.want)
			require.Empty(t, svc.deletedPermID, "no revoke may happen on validation failure")
		})
	}
}

func TestUnsharePropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUnshareCmd, svc, "json"), "file_1", "--email", "alice@example.com")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestUnshareRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUnshareCmd, svc, "json"), "--permission", "perm_1")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
