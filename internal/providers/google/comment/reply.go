package comment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newReplyCmd returns `comment reply`: a plain-text reply to one comment of
// the file, reporting the created reply.
func newReplyCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	var commentID, text string
	cmd := &cobra.Command{
		Use:   "reply <file-id>",
		Short: "Reply to a comment on a Doc or presentation",
		Example: `# Reply to a comment on a document
everything-cli google docs comment reply 1AbCdEfGh --comment comment_1 --text "Source added in v2"

# Reply to a comment on a presentation
everything-cli google slides comment reply 1AbCpresentationID --comment comment_1 --text "Fixed on slide 4"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--comment is required: the id of the comment to reply to (see comment list)")
			}
			if text == "" {
				return fmt.Errorf("--text is required: the reply text")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.CreateReply(cmd.Context(), args[0], commentID, text, "")
			if err != nil {
				return err
			}
			printReplyView(cmd, cfg, created)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&commentID, "comment", "", "Id of the comment to reply to (required)")
	f.StringVar(&text, "text", "", "Text of the reply (required)")
	return cmd
}
