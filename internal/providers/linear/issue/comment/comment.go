// Package comment builds the `linear issue comment` command tree: write
// verbs for an issue's comments. It deliberately does not import the sibling
// `issue` package — that parent will import this one, and importing back
// would create a cycle — so it defines its own view types.
package comment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// refView is the rendered shape of a comment's author.
type refView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// commentView is the rendered shape of one created comment: output field
// names are snake_case per the casing rule. A reply carries its parent
// comment's ID; top-level comments leave ParentID empty so the JSON key is
// omitted.
type commentView struct {
	ID        string   `json:"id"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	ParentID  string   `json:"parent_id,omitempty"`
	User      *refView `json:"user,omitempty"`
}

// commentFields are the table columns of one created comment.
var commentFields = []string{"created_at", "user", "body"}

// NewCmd returns the `comment` parent with its leaves attached. Every leaf
// lives in its own file with one AddCommand line per leaf.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage comments on a Linear issue",
	}
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	return cmd
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

// refName renders a possibly-absent reference as its display name.
func refName(r *refView) string {
	if r == nil {
		return ""
	}
	return r.Name
}

// commentTableRow flattens a comment view into table-row cells; the author
// reference renders as its display name.
func commentTableRow(v commentView) map[string]any {
	return map[string]any{
		"created_at": v.CreatedAt,
		"user":       refName(v.User),
		"body":       v.Body,
	}
}

// printComment renders one created comment. The view struct is passed as v
// so JSON/TOON render it as a single object with its json tags, while rows
// drive the table.
func printComment(cmd *cobra.Command, cfg *app.Config, c *service.Comment) {
	v := toCommentView(c)
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), commentFields, v,
		[]map[string]any{commentTableRow(v)})
}
