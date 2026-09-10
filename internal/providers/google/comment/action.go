package comment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newActionCmd returns a `comment <action>` leaf (resolve, reopen): it marks
// one comment via an action reply carrying no reply text, and reports the
// created reply. action is both the leaf name and the Drive API reply
// action, so the two leaves differ only in their action, Short, and Example.
func newActionCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService], action, short, example string) *cobra.Command {
	var commentID string
	cmd := &cobra.Command{
		Use:     action + " <file-id>",
		Short:   short,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--comment is required: the id of the comment to %s (see comment list)", action)
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.CreateReply(cmd.Context(), args[0], commentID, "", action)
			if err != nil {
				return err
			}
			printReplyView(cmd, cfg, created)
			return nil
		},
	}
	cmd.Flags().StringVar(&commentID, "comment", "", "Id of the comment to "+action+" (required)")
	return cmd
}
