package comment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newCreateCmd returns `linear issue comment create`: a comment on an issue,
// replying to --parent when given. --body is marked required so cobra
// rejects a missing body before RunE; the API also treats it as required.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	var (
		body   string
		parent string
	)
	cmd := &cobra.Command{
		Use:   "create <id>",
		Short: "Create a comment on a Linear issue",
		Example: `# Comment on issue BLA-123
everything-cli linear issue comment create BLA-123 --body "Looks good to me"

# Reply to an existing comment
everything-cli linear issue comment create BLA-123 --body "Agreed" --parent comment_1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			comment, err := svc.CreateComment(cmd.Context(), args[0], service.CreateCommentInput{
				Body:     body,
				ParentID: parent,
			})
			if err != nil {
				return err
			}
			printComment(cmd, cfg, comment)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "Comment body text (required)")
	f.StringVar(&parent, "parent", "", "Parent comment ID to reply to")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}
