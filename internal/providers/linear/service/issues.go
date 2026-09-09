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

// Issue is one Linear issue as decoded from the GraphQL API. The JSON tags
// are the wire shape (camelCase); leaves map it to snake_case views. A wire
// null decodes to the zero value (nil refs, empty timestamps).
type Issue struct {
	ID          string    `json:"id"`
	Identifier  string    `json:"identifier"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       *StateRef `json:"state"`
	Assignee    *NamedRef `json:"assignee"`
	Creator     *NamedRef `json:"creator"`
	Team        *Team     `json:"team"`
	URL         string    `json:"url"`
	CreatedAt   string    `json:"createdAt"`
	UpdatedAt   string    `json:"updatedAt"`
	StartedAt   string    `json:"startedAt"`
	CompletedAt string    `json:"completedAt"`
	CanceledAt  string    `json:"canceledAt"`
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
type IssueService interface {
	ListIssues(ctx context.Context, f IssueFilter) ([]Issue, error)
	ListComments(ctx context.Context, issueID string) ([]Comment, error)
	GetIssue(ctx context.Context, id string) (*Issue, error)
	CreateIssue(ctx context.Context, in CreateIssueInput) (*Issue, error)
	UpdateIssue(ctx context.Context, id string, in UpdateIssueInput) (*Issue, error)
}

// CreateIssueInput carries the fields of `linear issue create`. TeamID and
// Title are required (the CLI requires --title as UX even though the API's
// IssueCreateInput marks it nullable); empty optional fields are omitted
// from the mutation.
type CreateIssueInput struct {
	TeamID      string
	Title       string
	Description string
	AssigneeID  string
	StateID     string
}

// UpdateIssueInput carries the changed fields of `linear issue update`;
// empty fields are omitted from the mutation.
type UpdateIssueInput struct {
	Title       string
	Description string
	AssigneeID  string
	StateID     string
}

// issueFields is the selection set every issue query and mutation returns.
const issueFields = `id identifier title description url createdAt updatedAt
	startedAt completedAt canceledAt
	state { id name type } assignee { id name } creator { id name } team { id name key }`

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

// GetIssue returns one issue by UUID or human identifier ("BLA-123").
func (s *Service) GetIssue(ctx context.Context, id string) (*Issue, error) {
	const query = `query($id: String!) { issue(id: $id) { ` + issueFields + ` } }`
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
	return s.issuePayload(ctx, mutation, map[string]any{"input": input}, "issueCreate")
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
	return s.issuePayload(ctx, mutation, map[string]any{"id": id, "input": input}, "issueUpdate")
}

// issuePayload runs one issue mutation and returns its issue, failing when
// the payload reports success: false.
func (s *Service) issuePayload(ctx context.Context, query string, variables map[string]any, key string) (*Issue, error) {
	data, err := s.exec(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	raw, err := dig(data, key)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Success bool   `json:"success"`
		Issue   *Issue `json:"issue"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decoding %s payload: %w", key, err)
	}
	if !payload.Success || payload.Issue == nil {
		return nil, fmt.Errorf("linear API reported %s success: false", key)
	}
	return payload.Issue, nil
}
