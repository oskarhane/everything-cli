package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
)

// refView is the rendered shape of a linked object reference (state,
// assignee).
type refView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// teamView is the rendered shape of an issue's team.
type teamView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// issueView is the rendered shape of an issue: output field names are
// snake_case per the casing rule.
type issueView struct {
	ID          string    `json:"id"`
	Identifier  string    `json:"identifier"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	State       *refView  `json:"state,omitempty"`
	Assignee    *refView  `json:"assignee,omitempty"`
	Team        *teamView `json:"team,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// listFields are the table columns of issue list.
var listFields = []string{"identifier", "title", "state", "assignee", "team", "updated_at"}

// detailFields are the table columns of issue get and the mutation echoes.
var detailFields = []string{"id", "identifier", "title", "description", "state", "assignee", "team", "url", "created_at", "updated_at"}

// toView maps the wire issue to its rendered shape.
func toView(i *service.Issue) issueView {
	v := issueView{
		ID:          i.ID,
		Identifier:  i.Identifier,
		Title:       i.Title,
		Description: i.Description,
		URL:         i.URL,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}
	if i.State != nil {
		v.State = &refView{ID: i.State.ID, Name: i.State.Name}
	}
	if i.Assignee != nil {
		v.Assignee = &refView{ID: i.Assignee.ID, Name: i.Assignee.Name}
	}
	if i.Team != nil {
		v.Team = &teamView{ID: i.Team.ID, Name: i.Team.Name, Key: i.Team.Key}
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

// teamKey renders a possibly-absent team as its issue-prefix key.
func teamKey(t *teamView) string {
	if t == nil {
		return ""
	}
	return t.Key
}

// jsonRow renders a view as a full JSON/TOON row: nested references keep
// their id and name. It doubles as v for output.Print, so the
// one-row-vs-array convention applies to lists.
func jsonRow(v issueView) map[string]any {
	var state, assignee, team any
	if v.State != nil {
		state = map[string]any{"id": v.State.ID, "name": v.State.Name}
	}
	if v.Assignee != nil {
		assignee = map[string]any{"id": v.Assignee.ID, "name": v.Assignee.Name}
	}
	if v.Team != nil {
		team = map[string]any{"id": v.Team.ID, "name": v.Team.Name, "key": v.Team.Key}
	}
	return map[string]any{
		"id":          v.ID,
		"identifier":  v.Identifier,
		"title":       v.Title,
		"description": v.Description,
		"state":       state,
		"assignee":    assignee,
		"team":        team,
		"url":         v.URL,
		"created_at":  v.CreatedAt,
		"updated_at":  v.UpdatedAt,
	}
}

// tableRow flattens a view into table-row cells.
func tableRow(v issueView) map[string]any {
	return map[string]any{
		"id":          v.ID,
		"identifier":  v.Identifier,
		"title":       v.Title,
		"description": v.Description,
		"state":       refName(v.State),
		"assignee":    refName(v.Assignee),
		"team":        teamKey(v.Team),
		"url":         v.URL,
		"created_at":  v.CreatedAt,
		"updated_at":  v.UpdatedAt,
	}
}

// printIssueList renders an issue list under the one-row-vs-array output
// convention.
func printIssueList(cmd *cobra.Command, cfg *app.Config, issues []service.Issue) {
	jsonRows := make([]map[string]any, 0, len(issues))
	tableRows := make([]map[string]any, 0, len(issues))
	for i := range issues {
		v := toView(&issues[i])
		jsonRows = append(jsonRows, jsonRow(v))
		tableRows = append(tableRows, tableRow(v))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), listFields, jsonRows, tableRows)
}

// printIssue renders one issue (get, and the create/update echoes).
func printIssue(cmd *cobra.Command, cfg *app.Config, issue *service.Issue) {
	v := toView(issue)
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), detailFields, v,
		[]map[string]any{tableRow(v)})
}
