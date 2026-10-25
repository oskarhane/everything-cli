// Package image builds the `linear issue image` command tree: it discovers
// markdown image references in an issue's description (and comments) and
// downloads them from Linear's private file storage. It is a separate
// package from `issue` so `issue.NewCmd` can attach it without an import
// cycle; the scanner and filename rules keep that boundary intact.
package image

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// Dialers groups the per-surface dialers the image tree needs: the issue
// surface for descriptions, the comment surface for --comments, and the
// file surface for downloads.
type Dialers struct {
	Issue   service.Dialer[service.IssueService]
	Comment service.Dialer[service.CommentService]
	File    service.Dialer[service.FileService]
}

// NewCmd returns the `linear issue image` parent command with its leaves
// attached. Every leaf lives in its own file with one AddCommand line per
// leaf.
func NewCmd(cfg *app.Config, d Dialers) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Find and download images from a Linear issue",
	}
	cmd.AddCommand(newListCmd(cfg, d))
	cmd.AddCommand(newDownloadCmd(cfg, d))
	return cmd
}

// discoverRefs resolves one issue's image refs through the dialers: the
// description always, comment bodies too when withComments is set. Both
// image leaves build on it.
func discoverRefs(ctx context.Context, d Dialers, issueID string, withComments bool) ([]imageRef, error) {
	issueSvc, err := d.Issue(ctx)
	if err != nil {
		return nil, err
	}
	issue, err := issueSvc.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}
	var comments []service.Comment
	if withComments {
		commentSvc, err := d.Comment(ctx)
		if err != nil {
			return nil, err
		}
		if comments, err = commentSvc.ListComments(ctx, issueID); err != nil {
			return nil, err
		}
	}
	return discoverImages(issue.Description, comments), nil
}
