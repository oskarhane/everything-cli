package file

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newUnshareCmd returns `drive file unshare`: revoke one permission from a
// file, by --permission id or by --email (resolved via permissions.list).
// Exactly one of the two.
func newUnshareCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		permissionID string
		email        string
	)
	cmd := &cobra.Command{
		Use:   "unshare <file-id>",
		Short: "Revoke a permission from a Drive file",
		Example: `# Revoke a user's access by email
everything-cli drive file unshare 1AbCdEfGh --email alice@example.com

# Revoke one permission by id (see "drive file permissions" to find it)
everything-cli drive file unshare 1AbCdEfGh --permission 8zK`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (permissionID == "") == (email == "") {
				return fmt.Errorf("exactly one of --permission or --email is required")
			}
			permSvc, err := service.As[service.PermissionService](newSvc(cmd.Context()))
			if err != nil {
				return err
			}
			revoked := permissionID
			if email != "" {
				perms, err := permSvc.ListPermissions(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				perm, err := permissionForEmail(perms, email, args[0])
				if err != nil {
					return err
				}
				permissionID, revoked = perm.Id, email
			}
			if err := permSvc.DeletePermission(cmd.Context(), args[0], permissionID); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Revoked %s on %s\n", revoked, args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&permissionID, "permission", "", "Id of the permission to revoke (see `drive file permissions`)")
	f.StringVar(&email, "email", "", "Email address of the user whose access to revoke")
	return cmd
}

// permissionForEmail finds the one permission on fileID whose emailAddress
// matches case-insensitively. Zero matches names the email and points at the
// permissions leaf; multiple matches (e.g. the same address as both a user
// and a group) refuse and ask for --permission.
func permissionForEmail(perms []*drive.Permission, email, fileID string) (*drive.Permission, error) {
	var matches []*drive.Permission
	for _, p := range perms {
		if strings.EqualFold(p.EmailAddress, email) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no permission for %s on file %s: run \"everything-cli drive file permissions %s\" to find the permission id", email, fileID, fileID)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.Id)
		}
		return nil, fmt.Errorf("multiple permissions for %s on file %s (permission ids %s): pass --permission to choose one", email, fileID, strings.Join(ids, ", "))
	}
}
