package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue/comment"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newCommentsCmd returns `linear issue comments`: every comment on one
// issue by UUID or human identifier ("BLA-123"). Rendering is owned by the
// comment package; this leaf only dials and prints.
func newCommentsCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	return &cobra.Command{
		Use:   "comments <id>",
		Short: "List comments on a Linear issue",
		Example: `# List comments on issue BLA-123 as JSON
everything-cli linear issue comments BLA-123 --format json

# List comments on the same issue as a table
everything-cli linear issue comments 8c8a1b2c-0000-4000-8000-000000000001 --format table`,
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
			printCommentList(cmd, cfg, comments)
			return nil
		},
	}
}

// printCommentList renders a comment list under the one-row-vs-array
// output convention.
func printCommentList(cmd *cobra.Command, cfg *app.Config, comments []service.Comment) {
	jsonRows := make([]map[string]any, 0, len(comments))
	tableRows := make([]map[string]any, 0, len(comments))
	for i := range comments {
		v := comment.ToView(&comments[i])
		jsonRows = append(jsonRows, comment.JSONRow(v))
		tableRows = append(tableRows, comment.TableRow(v))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), comment.Fields, jsonRows, tableRows)
}
