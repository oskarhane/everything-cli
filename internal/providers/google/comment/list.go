package comment

import (
	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newListCmd returns `comment list`: every comment on the file, unresolved
// ones by default — the open questions are what a reader wants; --all adds
// the resolved ones.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "list <file-id>",
		Short: "List comments on a Doc or presentation",
		Example: `# List a document's unresolved comments as JSON
everything-cli google docs comment list 1AbCdEfGh --format json

# Include resolved comments too
everything-cli google slides comment list 1AbCpresentationID --all

# Same view as a table
everything-cli google docs comment list 1AbCdEfGh --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			comments, err := svc.ListComments(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if !all {
				comments = filterUnresolved(comments)
			}
			printCommentList(cmd, cfg, comments)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Include resolved comments, not just unresolved ones")
	return cmd
}

// filterUnresolved keeps the comments that are not resolved. A nil slice
// (a file with no comments) passes through as nil — zero rows, no panic.
func filterUnresolved(comments []*drive.Comment) []*drive.Comment {
	var kept []*drive.Comment
	for _, c := range comments {
		if !c.Resolved {
			kept = append(kept, c)
		}
	}
	return kept
}
