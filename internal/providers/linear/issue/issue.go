// Package issue builds the `linear issue` command tree.
package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue/attachment"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue/comment"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear issue` parent command with its leaves
// attached. Every leaf lives in its own file with one AddCommand line per
// leaf.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService], viewerSvc service.Dialer[service.ViewerService], commentSvc service.Dialer[service.CommentService], attachSvc service.Dialer[service.AttachmentService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage Linear issues",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc, viewerSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newUpdateCmd(cfg, newSvc))
	cmd.AddCommand(newCommentsCmd(cfg, newSvc))
	cmd.AddCommand(comment.NewCmd(cfg, commentSvc))
	cmd.AddCommand(attachment.NewCmd(cfg, attachSvc))
	return cmd
}
