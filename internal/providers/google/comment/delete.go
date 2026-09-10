package comment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newDeleteCmd returns `comment delete`: permanently remove one comment and
// its replies. Deletion cannot be undone, so it refuses to run without
// --force; a resolve reply is the recoverable alternative — it hides the
// comment from the default list without destroying it.
func newDeleteCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	var commentID string
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <file-id>",
		Short: "Delete a comment (destructive)",
		Example: `# See the refusal without --force
everything-cli google docs comment delete 1AbCdEfGh --comment comment_1

# Actually delete the comment
everything-cli google slides comment delete 1AbCpresentationID --comment comment_1 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--comment is required: the id of the comment to delete (see comment list)")
			}
			if !force {
				return fmt.Errorf("refusing to delete comment %q on file %q without --force (this cannot be undone; use \"%s\" instead)", commentID, args[0], resolveHint(cmd))
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteComment(cmd.Context(), args[0], commentID)
		},
	}
	f := cmd.Flags()
	f.StringVar(&commentID, "comment", "", "Id of the comment to delete (required)")
	f.BoolVar(&force, "force", false, "Delete the comment instead of refusing")
	return cmd
}

// resolveHint names the resolve leaf under the same parent chain as the
// delete invocation, so the recovery hint is correct whether the comment
// subtree hangs under docs or slides; a detached leaf gets the neutral form.
func resolveHint(cmd *cobra.Command) string {
	if parent := cmd.Parent(); parent != nil {
		return parent.CommandPath() + " resolve <file-id> --comment <id>"
	}
	return "comment resolve <file-id> --comment <id>"
}
