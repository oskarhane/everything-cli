package comment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newAddCmd returns `comment add`: a new top-level comment on the file as a
// whole (no anchor), reporting the created comment's id — the same report
// style drive file create uses.
func newAddCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	var text string
	cmd := &cobra.Command{
		Use:   "add <file-id>",
		Short: "Add a comment to a Doc or presentation",
		Example: `# Comment on a document
everything-cli google docs comment add 1AbCdEfGh --text "Needs a source for this claim"

# Comment on a presentation
everything-cli google slides comment add 1AbCpresentationID --text "Check slide 4's numbers"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if text == "" {
				return fmt.Errorf("--text is required: the comment text")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.CreateComment(cmd.Context(), args[0], text)
			if err != nil {
				return err
			}
			printCommentView(cmd, cfg, created)
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Text of the comment (required)")
	return cmd
}
