package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// State is one Linear workflow state — a column on a team's board — as
// decoded from the GraphQL API. Position orders the state within its team;
// Type is the workflow category: "triage", "backlog", "unstarted",
// "started", "completed", "canceled", or "duplicate".
type State struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Position float64 `json:"position"`
}

// StateService is the workflow-state surface the `linear state` subtree
// consumes.
type StateService interface {
	ListStates(ctx context.Context, teamID string) ([]State, error)
}

// Compile-time proof that Service satisfies the state surface. Each concern
// file owns its own seam assertion.
var _ StateService = (*Service)(nil)

// ListStates lists the workflow states of one team, ordered by Position
// ascending (Linear does not guarantee wire order). Team.states is a
// non-paginated connection, so this is exactly one request.
func (s *Service) ListStates(ctx context.Context, teamID string) ([]State, error) {
	const query = `query($id: String!) {
		team(id: $id) {
			states { nodes { id name position type } }
		}
	}`
	data, err := s.exec(ctx, query, map[string]any{"id": teamID})
	if err != nil {
		return nil, err
	}
	raw, err := dig(data, "team")
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("team %q not found", teamID)
	}
	var payload struct {
		States struct {
			Nodes []State `json:"nodes"`
		} `json:"states"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decoding team states: %w", err)
	}
	states := payload.States.Nodes
	sort.SliceStable(states, func(i, j int) bool {
		return states[i].Position < states[j].Position
	})
	return states, nil
}
