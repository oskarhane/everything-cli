// Package comment builds the shared `comment` command subtree: comments on
// Google Docs and Slides files are managed through the Drive API, so one
// subtree serves both, attached under the docs and slides parents.
package comment

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// NewCmd returns the `comment` parent with every comment leaf attached. Each
// leaf lives in its own file, and the parent hands each one a narrowed
// CommentService dialer — the same construction docs and slides use for
// their own leaves.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage comments on a Doc or presentation",
	}
	newSvc := func(ctx context.Context) (service.DriveService, error) {
		return dial(ctx, cfg)
	}
	commentSvc := func(ctx context.Context) (service.CommentService, error) {
		return service.As[service.CommentService](newSvc(ctx))
	}
	cmd.AddCommand(newListCmd(cfg, commentSvc))
	cmd.AddCommand(newAddCmd(cfg, commentSvc))
	cmd.AddCommand(newReplyCmd(cfg, commentSvc))
	cmd.AddCommand(newResolveCmd(cfg, commentSvc))
	cmd.AddCommand(newReopenCmd(cfg, commentSvc))
	cmd.AddCommand(newDeleteCmd(cfg, commentSvc))
	return cmd
}
