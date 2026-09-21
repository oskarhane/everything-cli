// Package issue builds the `linear issue` command tree.
package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue/attachment"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue/comment"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// Dialers groups the per-surface dialers the issue tree needs, so adding
// another subtree does not lengthen NewCmd's parameter list.
type Dialers struct {
	Issue      service.Dialer[service.IssueService]
	Viewer     service.Dialer[service.ViewerService]
	Comment    service.Dialer[service.CommentService]
	Attachment service.Dialer[service.AttachmentService]
	State      service.Dialer[service.StateService]
}

// NewCmd returns the `linear issue` parent command with its leaves
// attached. Every leaf lives in its own file with one AddCommand line per
// leaf.
func NewCmd(cfg *app.Config, d Dialers) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage Linear issues",
	}
	cmd.AddCommand(newListCmd(cfg, d.Issue, d.Viewer))
	cmd.AddCommand(newGetCmd(cfg, d.Issue))
	cmd.AddCommand(newCreateCmd(cfg, d.Issue, d.State))
	cmd.AddCommand(newUpdateCmd(cfg, d.Issue, d.State))
	cmd.AddCommand(newCommentsCmd(cfg, d.Comment))
	cmd.AddCommand(comment.NewCmd(cfg, d.Comment))
	cmd.AddCommand(attachment.NewCmd(cfg, d.Attachment))
	return cmd
}
