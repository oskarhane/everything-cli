package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// stateNode returns a mock workflow-state node.
func stateNode(id, name, typ string, position float64) map[string]any {
	return map[string]any{"id": id, "name": name, "type": typ, "position": position}
}

// TestListStates covers the single-request decode and the position-ascending
// ordering: the wire returns states out of order and every field decodes.
func TestListStates(t *testing.T) {
	tests := []struct {
		name string
		team string
		// nodes is the wire order; wantIDs is the expected position order.
		nodes   []any
		wantIDs []string
		want    []State
	}{
		{
			name: "sorts out-of-order wire states by position",
			team: "team_1",
			nodes: []any{
				stateNode("state_2", "In Progress", "started", 2),
				stateNode("state_1", "Todo", "unstarted", 1),
				stateNode("state_3", "Done", "completed", 3),
			},
			wantIDs: []string{"state_1", "state_2", "state_3"},
			want: []State{
				{ID: "state_1", Name: "Todo", Type: "unstarted", Position: 1},
				{ID: "state_2", Name: "In Progress", Type: "started", Position: 2},
				{ID: "state_3", Name: "Done", Type: "completed", Position: 3},
			},
		},
		{
			name:    "empty states decode to no rows",
			team:    "team_2",
			nodes:   []any{},
			wantIDs: []string{},
			want:    []State{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := mockGraphQL(t, func(gqlCall) any {
				return map[string]any{"team": map[string]any{"states": map[string]any{"nodes": tt.nodes}}}
			})
			svc := newTestService(srv)

			states, err := svc.ListStates(context.Background(), tt.team)
			require.NoError(t, err)
			require.Equal(t, tt.want, states)
			require.Equal(t, tt.wantIDs, stateIDs(states))

			// Exactly one request; states is non-paginated so only the team
			// id is sent, and the selection set is the agreed one.
			require.Len(t, *calls, 1)
			require.Equal(t, map[string]any{"id": tt.team}, (*calls)[0].Variables)
			require.Contains(t, (*calls)[0].Query, "team(id: $id)")
			require.Contains(t, (*calls)[0].Query, "states { nodes { id name position type } }")
		})
	}
}

// TestListStatesTeamNotFound asserts a null team payload is a not-found error
// naming the team id, not an empty list.
func TestListStatesTeamNotFound(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"team": nil}
	})
	svc := newTestService(srv)

	_, err := svc.ListStates(context.Background(), "team_999")
	require.ErrorContains(t, err, "not found")
	require.ErrorContains(t, err, "team_999")
	require.Len(t, *calls, 1)
}

func stateIDs(states []State) []string {
	ids := make([]string, len(states))
	for i, s := range states {
		ids[i] = s.ID
	}
	return ids
}
