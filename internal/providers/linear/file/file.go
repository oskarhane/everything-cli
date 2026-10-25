// Package file builds the `linear file` command subtree: downloads from
// Linear's file storage (uploads.linear.app), where issue image uploads
// live.
package file

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear file` parent command with its leaves attached.
// Every leaf lives in its own file with one AddCommand line per leaf.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Download files from Linear file storage",
	}
	cmd.AddCommand(newDownloadCmd(cfg, newSvc))
	return cmd
}
