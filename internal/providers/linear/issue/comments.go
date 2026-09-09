package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// commentView is the rendered shape of one issue comment: output field
// names are snake_case per the casing rule. A reply carries its parent
// comment's ID; top-level comments leave ParentID empty so the JSON key
// is omitted.
type commentView struct {
	ID        string   `json:"id"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	ParentID  string   `json:"parent_id,omitempty"`
	User      *refView `json:"user,omitempty"`
}

// commentFields are the table columns of issue comments.
var commentFields = []string{"created_at", "user", "body"}

// newCommentsCmd returns `linear issue comments`: every comment on one
// issue by UUID or human identifier ("BLA-123").
func newCommentsCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService]) *cobra.Command {
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

// toCommentView maps a wire comment to its rendered shape.
func toCommentView(c *service.Comment) commentView {
	v := commentView{
		ID:        c.ID,
		Body:      c.Body,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if c.Parent != nil {
		v.ParentID = c.Parent.ID
	}
	if c.User != nil {
		v.User = &refView{ID: c.User.ID, Name: c.User.Name}
	}
	return v
}

// commentJSONRow renders a comment view as a full JSON/TOON row: the user
// reference keeps its id and name; parent_id appears only on replies.
func commentJSONRow(v commentView) map[string]any {
	row := map[string]any{
		"id":         v.ID,
		"body":       v.Body,
		"created_at": v.CreatedAt,
		"updated_at": v.UpdatedAt,
	}
	if v.ParentID != "" {
		row["parent_id"] = v.ParentID
	}
	if v.User != nil {
		row["user"] = map[string]any{"id": v.User.ID, "name": v.User.Name}
	}
	return row
}

// commentTableRow flattens a comment view into table-row cells; the user
// reference renders as its display name.
func commentTableRow(v commentView) map[string]any {
	return map[string]any{
		"created_at": v.CreatedAt,
		"user":       refName(v.User),
		"body":       v.Body,
	}
}

// printCommentList renders a comment list under the one-row-vs-array
// output convention.
func printCommentList(cmd *cobra.Command, cfg *app.Config, comments []service.Comment) {
	jsonRows := make([]map[string]any, 0, len(comments))
	tableRows := make([]map[string]any, 0, len(comments))
	for i := range comments {
		v := toCommentView(&comments[i])
		jsonRows = append(jsonRows, commentJSONRow(v))
		tableRows = append(tableRows, commentTableRow(v))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), commentFields, jsonRows, tableRows)
}
