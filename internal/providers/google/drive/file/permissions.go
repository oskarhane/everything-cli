package file

import (
	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// permissionFields is the permission row field order for table output; the
// same names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var permissionFields = []string{"id", "type", "role", "email_address", "display_name", "deleted"}

// permissionRow maps one permission to its output row. email_address renders
// "" for anyone/domain permissions (the API does not set it there).
func permissionRow(p *drive.Permission) map[string]any {
	return map[string]any{
		"id":            p.Id,
		"type":          p.Type,
		"role":          p.Role,
		"email_address": p.EmailAddress,
		"display_name":  p.DisplayName,
		"deleted":       p.Deleted,
	}
}

// printPermissions renders zero or more permissions: a JSON/TOON array (or a
// single object for one row), or a table, in the resolved output format.
func printPermissions(cmd *cobra.Command, cfg *app.Config, perms []*drive.Permission) {
	rows := make([]map[string]any, 0, len(perms))
	for _, p := range perms {
		rows = append(rows, permissionRow(p))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), permissionFields, rows, rows)
}

// newPermissionsCmd returns `drive file permissions`: every permission on a
// file. Run it to find the permission id unshare needs.
func newPermissionsCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions <file-id>",
		Short: "List the permissions on a Drive file",
		Example: `# List who a file is shared with, as a table
everything-cli drive file permissions 1AbCdEfGh --format table

# List the same permissions as JSON
everything-cli drive file permissions 1AbCdEfGh --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permSvc, err := service.As[service.PermissionService](newSvc(cmd.Context()))
			if err != nil {
				return err
			}
			perms, err := permSvc.ListPermissions(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printPermissions(cmd, cfg, perms)
			return nil
		},
	}
	return cmd
}
