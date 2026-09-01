// Package file builds the `drive file` command subtree.
package file

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// NewCmd returns the `drive file` parent with every file leaf attached.
// Each leaf lives in its own file: list.go, get.go, create.go, upload.go,
// download.go, trash.go, untrash.go, delete.go, and the sharing leaves
// (permissions, share, unshare), one AddCommand line each.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage files in Drive",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newUploadCmd(cfg, newSvc))
	cmd.AddCommand(newDownloadCmd(cfg, newSvc))
	cmd.AddCommand(newTrashCmd(cfg, newSvc))
	cmd.AddCommand(newUntrashCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newPermissionsCmd(cfg, newSvc))
	cmd.AddCommand(newShareCmd(cfg, newSvc))
	cmd.AddCommand(newUnshareCmd(cfg, newSvc))
	return cmd
}
