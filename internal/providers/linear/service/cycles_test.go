package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// cycleNode returns a mock cycle node.
func cycleNode(id, name string, number int) map[string]any {
	return map[string]any{"id": id, "name": name, "number": number}
}

// TestListCyclesFollowsCursorAcrossTwoPages covers the multi-page decode:
// both pages' cycles are returned, the second request follows
// pageInfo.endCursor, and the team id rides along on every page. A wire
// null name (unnamed cycle) decodes to "".
func TestListCyclesFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"team": map[string]any{"cycles": conn([]any{
				cycleNode("cycle_1", "Sprint One", 1),
				map[string]any{"id": "cycle_2", "name": nil, "number": 2},
			}, true, "cursor-1")}}
		}
		return map[string]any{"team": map[string]any{"cycles": conn([]any{
			cycleNode("cycle_3", "Sprint Three", 3),
		}, false, "")}}
	})
	svc := newTestService(srv)

	cycles, err := svc.ListCycles(context.Background(), "team_1")
	require.NoError(t, err)
	require.Equal(t, []Cycle{
		{ID: "cycle_1", Name: "Sprint One", Number: 1},
		{ID: "cycle_2", Name: "", Number: 2},
		{ID: "cycle_3", Name: "Sprint Three", Number: 3},
	}, cycles)

	// Two requests: the second follows pageInfo.endCursor, and both send
	// the team id and page size.
	require.Len(t, *calls, 2)
	require.NotContains(t, (*calls)[0].Variables, "after")
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	for _, c := range *calls {
		require.Equal(t, "team_1", c.Variables["id"])
		require.Equal(t, float64(pageSize), c.Variables["first"])
	}
	require.Contains(t, (*calls)[0].Query, "team(id: $id)")
	require.Contains(t, (*calls)[0].Query, "nodes { id name number }")
}

// TestListCyclesEmpty asserts an empty connection decodes to no rows in a
// single request.
func TestListCyclesEmpty(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"team": map[string]any{"cycles": conn([]any{}, false, "")}}
	})
	svc := newTestService(srv)

	cycles, err := svc.ListCycles(context.Background(), "team_2")
	require.NoError(t, err)
	require.Empty(t, cycles)
	require.Len(t, *calls, 1)
}

// TestListCyclesTeamNotFound asserts a null team payload is an error, not
// an empty list.
func TestListCyclesTeamNotFound(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"team": nil}
	})
	svc := newTestService(srv)

	_, err := svc.ListCycles(context.Background(), "team_999")
	require.ErrorContains(t, err, "null")
	require.Len(t, *calls, 1)
}
