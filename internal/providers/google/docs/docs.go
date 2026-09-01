// Package docs builds the `docs` command tree.
package docs

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// NewCmd returns the `docs` parent command with its content leaves attached.
// The resource IS the document, so the leaves hang directly off this parent
// (flat, like calendar's CRUD leaves); each leaf lives in its own file.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Manage Google Docs content",
	}
	newSvc := func(ctx context.Context) (service.DriveService, error) {
		return dial(ctx, cfg)
	}
	// The concrete service implements every drive interface; As narrows the
	// shared seam to each leaf's own surface. get/append/insert/replace ride
	// the Docs API (DocService); delete is a Drive operation (FileService).
	cmd.AddCommand(newGetCmd(cfg, func(ctx context.Context) (service.DocService, error) {
		return service.As[service.DocService](newSvc(ctx))
	}))
	cmd.AddCommand(newAppendCmd(cfg, func(ctx context.Context) (service.DocService, error) {
		return service.As[service.DocService](newSvc(ctx))
	}))
	cmd.AddCommand(newInsertCmd(cfg, func(ctx context.Context) (service.DocService, error) {
		return service.As[service.DocService](newSvc(ctx))
	}))
	cmd.AddCommand(newReplaceCmd(cfg, func(ctx context.Context) (service.DocService, error) {
		return service.As[service.DocService](newSvc(ctx))
	}))
	cmd.AddCommand(newDeleteCmd(cfg, func(ctx context.Context) (service.FileService, error) {
		return service.As[service.FileService](newSvc(ctx))
	}))
	return cmd
}
