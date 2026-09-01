// Package drive builds the `drive` command tree.
package drive

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/file"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// NewCmd returns the `drive` parent command with its subtrees attached.
// Each subtree lives in its own dir; every leaf lives in its own file with
// one AddCommand line per leaf.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drive",
		Short: "Manage Google Drive files and their sharing",
	}
	newSvc := func(ctx context.Context) (service.DriveService, error) {
		return dial(ctx, cfg)
	}
	// The concrete service implements every drive interface; As narrows the
	// shared seam to each subtree's own surface.
	cmd.AddCommand(file.NewCmd(cfg, func(ctx context.Context) (service.FileService, error) {
		return service.As[service.FileService](newSvc(ctx))
	}))
	return cmd
}
