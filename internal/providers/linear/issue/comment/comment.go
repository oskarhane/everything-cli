// Package comment builds the `linear issue comment` command tree: write
// verbs for an issue's comments. It is also the single owner of comment
// rendering — the parent `issue` package's comments list leaf consumes the
// exported view surface — so both copies of the output shape cannot drift.
// It deliberately does not import the sibling `issue` package: that parent
// imports this one, and importing back would create a cycle.
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

// View is the rendered shape of one comment: output field names are
// snake_case per the casing rule. A reply carries its parent comment's ID;
// top-level comments leave ParentID empty so the JSON key is omitted.
type View struct {
	ID        string   `json:"id"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	ParentID  string   `json:"parent_id,omitempty"`
	User      *refView `json:"user,omitempty"`
}

// Fields are the table columns of a comment.
var Fields = []string{"created_at", "user", "body"}

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

// ToView maps a wire comment to its rendered shape.
func ToView(c *service.Comment) View {
	v := View{
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

// JSONRow renders a comment view as a full JSON/TOON row: the user
// reference keeps its id and name; parent_id appears only on replies.
func JSONRow(v View) map[string]any {
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

// TableRow flattens a comment view into table-row cells; the author
// reference renders as its display name.
func TableRow(v View) map[string]any {
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
	v := ToView(c)
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), Fields, v,
		[]map[string]any{TableRow(v)})
}
