package service

import (
	"context"
	"encoding/json"
	"fmt"
)

// NamedRef is a linked Linear object reference (assignee, creator).
type NamedRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// StateRef is a linked Linear workflow state reference, carrying the
// state's type ("started", "completed", "canceled", ...).
type StateRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// IssueRef is a compact issue reference used for parent/children links:
// enough to render a mention without embedding a full issue.
type IssueRef struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// Issue is one Linear issue as decoded from the GraphQL API. The JSON tags
// are the wire shape (camelCase); leaves map it to snake_case views. A wire
// null decodes to the zero value (nil refs, empty timestamps). The detail
// fields (children, priority, due date, estimate, cycle, milestone) are only
// populated by GetIssue's wider selection set; list and mutation payloads
// leave them zero.
type Issue struct {
	ID            string     `json:"id"`
	Identifier    string     `json:"identifier"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	State         *StateRef  `json:"state"`
	Assignee      *NamedRef  `json:"assignee"`
	Creator       *NamedRef  `json:"creator"`
	Team          *Team      `json:"team"`
	URL           string     `json:"url"`
	CreatedAt     string     `json:"createdAt"`
	UpdatedAt     string     `json:"updatedAt"`
	StartedAt     string     `json:"startedAt"`
	CompletedAt   string     `json:"completedAt"`
	CanceledAt    string     `json:"canceledAt"`
	Parent        *IssueRef  `json:"parent"`
	Children      []IssueRef `json:"-"`
	Priority      int        `json:"priority"`
	PriorityLabel string     `json:"priorityLabel"`
	DueDate       string     `json:"dueDate"`
	Estimate      *int       `json:"estimate"`
	Cycle         *NamedRef  `json:"cycle"`
	Milestone     *NamedRef  `json:"projectMilestone"`
}

// UnmarshalJSON flattens the children Relay connection (`children { nodes }`)
// into the flat Children slice; every other field decodes by tag.
func (i *Issue) UnmarshalJSON(data []byte) error {
	type plain Issue
	var wire struct {
		plain
		Children struct {
			Nodes []IssueRef `json:"nodes"`
		} `json:"children"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*i = Issue(wire.plain)
	i.Children = wire.Children.Nodes
	return nil
}

// IssueFilter narrows a ListIssues call. Every field is optional; set
// fields compose into a single GraphQL filter object. UpdatedSince is a
// pre-parsed RFC3339 timestamp compared against updatedAt.
type IssueFilter struct {
	TeamID       string
	AssigneeID   string
	CreatorID    string
	UpdatedSince string
}

// filterMap builds the GraphQL IssueFilter variable from f's set fields:
// team/assignee/creator match by id equality, UpdatedSince matches
// updatedAt at-or-after. The zero filter returns nil so the variable is
// omitted from the request entirely.
func (f IssueFilter) filterMap() map[string]any {
	var m map[string]any
	idEq := func(key, id string) {
		if id == "" {
			return
		}
		if m == nil {
			m = map[string]any{}
		}
		m[key] = map[string]any{"id": map[string]any{"eq": id}}
	}
	idEq("team", f.TeamID)
	idEq("assignee", f.AssigneeID)
	idEq("creator", f.CreatorID)
	if f.UpdatedSince != "" {
		if m == nil {
			m = map[string]any{}
		}
		m["updatedAt"] = map[string]any{"gte": f.UpdatedSince}
	}
	return m
}

// IssueService is the issue surface the `linear issue` subtree consumes.
// Comments live on CommentService — see comments.go.
type IssueService interface {
	ListIssues(ctx context.Context, f IssueFilter) ([]Issue, error)
	GetIssue(ctx context.Context, id string) (*Issue, error)
	CreateIssue(ctx context.Context, in CreateIssueInput) (*Issue, error)
	UpdateIssue(ctx context.Context, id string, in UpdateIssueInput) (*Issue, error)
}

// CreateIssueInput carries the fields of `linear issue create`. TeamID and
// Title are required (the CLI requires --title as UX even though the API's
// IssueCreateInput marks it nullable); empty optional fields are omitted
// from the mutation. Priority 0 is the API's "no priority" default, so a
// zero Priority is omitted rather than sent.
type CreateIssueInput struct {
	TeamID             string
	Title              string
	Description        string
	AssigneeID         string
	StateID            string
	ProjectID          string
	ParentID           string
	LabelIDs           []string
	Priority           int
	DueDate            string
	Estimate           int
	CycleID            string
	ProjectMilestoneID string
}

// UpdateIssueInput carries the changed fields of `linear issue update`.
// Empty string fields are omitted from the mutation. The pointer fields
// distinguish omit from clear: nil omits the key, a non-nil pointer sends
// it — an empty string sends null (un-parent, clear the due date, leave the
// cycle/milestone) and an empty LabelIDs slice sends [] (remove all labels).
type UpdateIssueInput struct {
	Title              string
	Description        string
	AssigneeID         string
	StateID            string
	ProjectID          string
	ParentID           *string
	LabelIDs           *[]string
	Priority           *int
	DueDate            *string
	Estimate           *int
	CycleID            *string
	ProjectMilestoneID *string
}

// issueFields is the selection set every issue query and mutation returns.
const issueFields = `id identifier title description url createdAt updatedAt
	startedAt completedAt canceledAt
	state { id name type } assignee { id name } creator { id name } team { id name key }
	parent { id identifier title }`

// issueDetailFields is GetIssue's wider selection set: the shared fields
// plus the detail-only fields (children, priority, due date, estimate,
// cycle, milestone) that list and mutation payloads do not need.
const issueDetailFields = issueFields + `
	children { nodes { id identifier title } } priority priorityLabel dueDate estimate
	cycle { id name } projectMilestone { id name }`

// ListIssues lists issues, most recently updated first. A zero filter lists
// the whole workspace; set filter fields compose into one GraphQL filter.
func (s *Service) ListIssues(ctx context.Context, f IssueFilter) ([]Issue, error) {
	const query = `query($filter: IssueFilter, $first: Int, $after: String) {
		issues(filter: $filter, first: $first, after: $after, orderBy: updatedAt) {
			nodes { ` + issueFields + ` }
			pageInfo { hasNextPage endCursor }
		}
	}`
	var variables map[string]any
	if filter := f.filterMap(); filter != nil {
		variables = map[string]any{"filter": filter}
	}
	return collectPages[Issue](ctx, s, query, variables, "issues")
}

// GetIssue returns one issue by UUID or human identifier ("BLA-123"),
// selecting the full detail field set.
func (s *Service) GetIssue(ctx context.Context, id string) (*Issue, error) {
	const query = `query($id: String!) { issue(id: $id) { ` + issueDetailFields + ` } }`
	data, err := s.exec(ctx, query, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	raw, err := dig(data, "issue")
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("issue %q not found", id)
	}
	var issue Issue
	if err := json.Unmarshal(raw, &issue); err != nil {
		return nil, fmt.Errorf("decoding issue: %w", err)
	}
	return &issue, nil
}

// CreateIssue creates an issue and returns it as created.
func (s *Service) CreateIssue(ctx context.Context, in CreateIssueInput) (*Issue, error) {
	const mutation = `mutation($input: IssueCreateInput!) {
		issueCreate(input: $input) { success issue { ` + issueFields + ` } }
	}`
	input := map[string]any{"teamId": in.TeamID, "title": in.Title}
	if in.Description != "" {
		input["description"] = in.Description
	}
	if in.AssigneeID != "" {
		input["assigneeId"] = in.AssigneeID
	}
	if in.StateID != "" {
		input["stateId"] = in.StateID
	}
	if in.ProjectID != "" {
		input["projectId"] = in.ProjectID
	}
	if in.ParentID != "" {
		input["parentId"] = in.ParentID
	}
	if len(in.LabelIDs) > 0 {
		input["labelIds"] = in.LabelIDs
	}
	if in.Priority != 0 {
		input["priority"] = in.Priority
	}
	if in.DueDate != "" {
		input["dueDate"] = in.DueDate
	}
	if in.Estimate != 0 {
		input["estimate"] = in.Estimate
	}
	if in.CycleID != "" {
		input["cycleId"] = in.CycleID
	}
	if in.ProjectMilestoneID != "" {
		input["projectMilestoneId"] = in.ProjectMilestoneID
	}
	return mutationPayload[Issue](ctx, s, mutation, map[string]any{"input": input}, "issueCreate", "issue")
}

// UpdateIssue updates the issue id (UUID or "BLA-123") with the non-empty
// fields of in, and returns it as updated.
func (s *Service) UpdateIssue(ctx context.Context, id string, in UpdateIssueInput) (*Issue, error) {
	const mutation = `mutation($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) { success issue { ` + issueFields + ` } }
	}`
	input := map[string]any{}
	if in.Title != "" {
		input["title"] = in.Title
	}
	if in.Description != "" {
		input["description"] = in.Description
	}
	if in.AssigneeID != "" {
		input["assigneeId"] = in.AssigneeID
	}
	if in.StateID != "" {
		input["stateId"] = in.StateID
	}
	if in.ProjectID != "" {
		input["projectId"] = in.ProjectID
	}
	// Pointer fields: nil omits the key; a pointer to "" clears the link by
	// sending null (Linear clears parent/dueDate/cycle/milestone on null).
	setNullable := func(key string, v *string) {
		if v == nil {
			return
		}
		if *v == "" {
			input[key] = nil
			return
		}
		input[key] = *v
	}
	setNullable("parentId", in.ParentID)
	setNullable("dueDate", in.DueDate)
	setNullable("cycleId", in.CycleID)
	setNullable("projectMilestoneId", in.ProjectMilestoneID)
	if in.LabelIDs != nil {
		// A pointer to an empty (or nil) slice sends labelIds: [], which
		// removes every label; normalize so the wire value is never null.
		ids := *in.LabelIDs
		if ids == nil {
			ids = []string{}
		}
		input["labelIds"] = ids
	}
	if in.Priority != nil {
		input["priority"] = *in.Priority
	}
	if in.Estimate != nil {
		input["estimate"] = *in.Estimate
	}
	return mutationPayload[Issue](ctx, s, mutation, map[string]any{"id": id, "input": input}, "issueUpdate", "issue")
}
