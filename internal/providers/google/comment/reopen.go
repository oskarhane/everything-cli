package comment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newReopenCmd returns `comment reopen`: mark one resolved comment open again
// via a reopen action reply (no reply text), reporting the created reply.
func newReopenCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	var commentID string
	cmd := &cobra.Command{
		Use:   "reopen <file-id>",
		Short: "Reopen a resolved comment",
		Example: `# Reopen a comment on a document
everything-cli google docs comment reopen 1AbCdEfGh --comment comment_1

# Reopen a comment on a presentation
everything-cli google slides comment reopen 1AbCpresentationID --comment comment_1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--comment is required: the id of the comment to reopen (see comment list)")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.CreateReply(cmd.Context(), args[0], commentID, "", "reopen")
			if err != nil {
				return err
			}
			printReplyView(cmd, cfg, created)
			return nil
		},
	}
	cmd.Flags().StringVar(&commentID, "comment", "", "Id of the comment to reopen (required)")
	return cmd
}
