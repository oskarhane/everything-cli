// Package slides builds the `slides` command tree.
package slides

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// NewCmd returns the `slides` parent with its leaves attached. get and
// replace ride the SlideService seam, delete the FileService one; both are
// narrowed from the one dial by As, since the concrete drive service
// implements every interface.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slides",
		Short: "Manage Google Slides presentations",
	}
	newSvc := func(ctx context.Context) (service.DriveService, error) {
		return dial(ctx, cfg)
	}
	cmd.AddCommand(newGetCmd(cfg, func(ctx context.Context) (service.SlideService, error) {
		return service.As[service.SlideService](newSvc(ctx))
	}))
	cmd.AddCommand(newReplaceCmd(cfg, func(ctx context.Context) (service.SlideService, error) {
		return service.As[service.SlideService](newSvc(ctx))
	}))
	cmd.AddCommand(newDeleteCmd(cfg, func(ctx context.Context) (service.FileService, error) {
		return service.As[service.FileService](newSvc(ctx))
	}))
	return cmd
}
