package issue

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// refView is the rendered shape of a linked object reference (assignee,
// creator).
type refView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// stateView is the rendered shape of an issue's workflow state, including
// its type (unstarted/started/completed/canceled).
type stateView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// teamView is the rendered shape of an issue's team.
type teamView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// issueRefView is the rendered shape of a compact issue reference (parent,
// children): enough to render a mention without embedding a full issue.
type issueRefView struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// issueView is the rendered shape of an issue: output field names are
// snake_case per the casing rule.
type issueView struct {
	ID            string         `json:"id"`
	Identifier    string         `json:"identifier"`
	Title         string         `json:"title"`
	Description   string         `json:"description,omitempty"`
	State         *stateView     `json:"state,omitempty"`
	Assignee      *refView       `json:"assignee,omitempty"`
	Creator       *refView       `json:"creator,omitempty"`
	Team          *teamView      `json:"team,omitempty"`
	URL           string         `json:"url,omitempty"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	StartedAt     string         `json:"started_at,omitempty"`
	CompletedAt   string         `json:"completed_at,omitempty"`
	CanceledAt    string         `json:"canceled_at,omitempty"`
	Parent        *issueRefView  `json:"parent,omitempty"`
	Children      []issueRefView `json:"children,omitempty"`
	Priority      int            `json:"priority"`
	PriorityLabel string         `json:"priority_label,omitempty"`
	DueDate       string         `json:"due_date,omitempty"`
	Estimate      *int           `json:"estimate,omitempty"`
	Cycle         *refView       `json:"cycle,omitempty"`
	Milestone     *refView       `json:"milestone,omitempty"`
}

// listFields are the table columns of issue list.
var listFields = []string{"identifier", "title", "state", "assignee", "team", "updated_at"}

// detailFields are the table columns of issue get and the mutation echoes.
var detailFields = []string{"id", "identifier", "title", "description", "state", "assignee", "creator", "started_at", "completed_at", "canceled_at", "parent", "children", "priority", "due_date", "estimate", "cycle", "milestone", "team", "url", "created_at", "updated_at"}

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
		v.State = &stateView{ID: i.State.ID, Name: i.State.Name, Type: i.State.Type}
	}
	if i.Assignee != nil {
		v.Assignee = &refView{ID: i.Assignee.ID, Name: i.Assignee.Name}
	}
	if i.Creator != nil {
		v.Creator = &refView{ID: i.Creator.ID, Name: i.Creator.Name}
	}
	if i.Team != nil {
		v.Team = &teamView{ID: i.Team.ID, Name: i.Team.Name, Key: i.Team.Key}
	}
	v.StartedAt = i.StartedAt
	v.CompletedAt = i.CompletedAt
	v.CanceledAt = i.CanceledAt
	if i.Parent != nil {
		v.Parent = &issueRefView{ID: i.Parent.ID, Identifier: i.Parent.Identifier, Title: i.Parent.Title}
	}
	if len(i.Children) > 0 {
		v.Children = make([]issueRefView, 0, len(i.Children))
		for _, c := range i.Children {
			v.Children = append(v.Children, issueRefView{ID: c.ID, Identifier: c.Identifier, Title: c.Title})
		}
	}
	v.Priority = i.Priority
	v.PriorityLabel = i.PriorityLabel
	v.DueDate = i.DueDate
	v.Estimate = i.Estimate
	if i.Cycle != nil {
		v.Cycle = &refView{ID: i.Cycle.ID, Name: i.Cycle.Name}
	}
	if i.Milestone != nil {
		v.Milestone = &refView{ID: i.Milestone.ID, Name: i.Milestone.Name}
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

// stateName renders a possibly-absent state as its display name.
func stateName(s *stateView) string {
	if s == nil {
		return ""
	}
	return s.Name
}

// teamKey renders a possibly-absent team as its issue-prefix key.
func teamKey(t *teamView) string {
	if t == nil {
		return ""
	}
	return t.Key
}

// parentIdentifier renders a possibly-absent parent as its human identifier
// ("ENG-1").
func parentIdentifier(p *issueRefView) string {
	if p == nil {
		return ""
	}
	return p.Identifier
}

// childrenIdentifiers renders child references as a comma-separated
// identifier list ("ENG-2, ENG-3").
func childrenIdentifiers(children []issueRefView) string {
	ids := make([]string, 0, len(children))
	for _, c := range children {
		ids = append(ids, c.Identifier)
	}
	return strings.Join(ids, ", ")
}

// priorityCell renders the priority for table cells: the label when the API
// sent one, else the bare number — 0 (no priority) stays empty.
func priorityCell(v issueView) string {
	if v.PriorityLabel != "" {
		return v.PriorityLabel
	}
	if v.Priority != 0 {
		return strconv.Itoa(v.Priority)
	}
	return ""
}

// estimateCell renders a possibly-absent estimate for table cells.
func estimateCell(e *int) string {
	if e == nil {
		return ""
	}
	return strconv.Itoa(*e)
}

// jsonRow renders a view as a full JSON/TOON row: nested references keep
// their id and name. It doubles as v for output.Print, so the
// one-row-vs-array convention applies to lists.
func jsonRow(v issueView) map[string]any {
	var state, assignee, team any
	if v.State != nil {
		state = map[string]any{"id": v.State.ID, "name": v.State.Name, "type": v.State.Type}
	}
	if v.Assignee != nil {
		assignee = map[string]any{"id": v.Assignee.ID, "name": v.Assignee.Name}
	}
	if v.Team != nil {
		team = map[string]any{"id": v.Team.ID, "name": v.Team.Name, "key": v.Team.Key}
	}
	row := map[string]any{
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
	// Wire nulls decode to ""/nil; absent references and timestamps must not
	// surface as empty values in JSON/TOON.
	if v.Creator != nil {
		row["creator"] = map[string]any{"id": v.Creator.ID, "name": v.Creator.Name}
	}
	if v.StartedAt != "" {
		row["started_at"] = v.StartedAt
	}
	if v.CompletedAt != "" {
		row["completed_at"] = v.CompletedAt
	}
	if v.CanceledAt != "" {
		row["canceled_at"] = v.CanceledAt
	}
	// Detail-only fields: populated by GetIssue's wider selection set. Absent
	// references and empty values must not surface as empty keys.
	if v.Priority != 0 {
		row["priority"] = v.Priority
	}
	if v.PriorityLabel != "" {
		row["priority_label"] = v.PriorityLabel
	}
	if v.DueDate != "" {
		row["due_date"] = v.DueDate
	}
	if v.Estimate != nil {
		row["estimate"] = *v.Estimate
	}
	if v.Parent != nil {
		row["parent"] = map[string]any{"id": v.Parent.ID, "identifier": v.Parent.Identifier, "title": v.Parent.Title}
	}
	if len(v.Children) > 0 {
		children := make([]map[string]any, 0, len(v.Children))
		for _, c := range v.Children {
			children = append(children, map[string]any{"id": c.ID, "identifier": c.Identifier, "title": c.Title})
		}
		row["children"] = children
	}
	if v.Cycle != nil {
		row["cycle"] = map[string]any{"id": v.Cycle.ID, "name": v.Cycle.Name}
	}
	if v.Milestone != nil {
		row["milestone"] = map[string]any{"id": v.Milestone.ID, "name": v.Milestone.Name}
	}
	return row
}

// tableRow flattens a view into table-row cells.
func tableRow(v issueView) map[string]any {
	return map[string]any{
		"id":           v.ID,
		"identifier":   v.Identifier,
		"title":        v.Title,
		"description":  v.Description,
		"state":        stateName(v.State),
		"assignee":     refName(v.Assignee),
		"creator":      refName(v.Creator),
		"team":         teamKey(v.Team),
		"url":          v.URL,
		"created_at":   v.CreatedAt,
		"updated_at":   v.UpdatedAt,
		"started_at":   v.StartedAt,
		"completed_at": v.CompletedAt,
		"canceled_at":  v.CanceledAt,
		"parent":       parentIdentifier(v.Parent),
		"children":     childrenIdentifiers(v.Children),
		"priority":     priorityCell(v),
		"due_date":     v.DueDate,
		"estimate":     estimateCell(v.Estimate),
		"cycle":        refName(v.Cycle),
		"milestone":    refName(v.Milestone),
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
