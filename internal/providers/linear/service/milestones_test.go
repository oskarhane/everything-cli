package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// milestoneNode returns a mock project-milestone node.
func milestoneNode(id, name string) map[string]any {
	return map[string]any{"id": id, "name": name}
}

// TestListProjectMilestonesFollowsCursorAcrossTwoPages covers the
// multi-page decode: both pages' milestones are returned, the second
// request follows pageInfo.endCursor, and the project id rides along on
// every page.
func TestListProjectMilestonesFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"project": map[string]any{"projectMilestones": conn([]any{
				milestoneNode("milestone_1", "Alpha"),
			}, true, "cursor-1")}}
		}
		return map[string]any{"project": map[string]any{"projectMilestones": conn([]any{
			milestoneNode("milestone_2", "Beta"),
		}, false, "")}}
	})
	svc := newTestService(srv)

	milestones, err := svc.ListProjectMilestones(context.Background(), "project_1")
	require.NoError(t, err)
	require.Equal(t, []Milestone{
		{ID: "milestone_1", Name: "Alpha"},
		{ID: "milestone_2", Name: "Beta"},
	}, milestones)

	// Two requests: the second follows pageInfo.endCursor, and both send
	// the project id and page size.
	require.Len(t, *calls, 2)
	require.NotContains(t, (*calls)[0].Variables, "after")
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	for _, c := range *calls {
		require.Equal(t, "project_1", c.Variables["id"])
		require.Equal(t, float64(pageSize), c.Variables["first"])
	}
	require.Contains(t, (*calls)[0].Query, "project(id: $id)")
	require.Contains(t, (*calls)[0].Query, "projectMilestones(first: $first, after: $after)")
	require.Contains(t, (*calls)[0].Query, "nodes { id name }")
}

// TestListProjectMilestonesEmpty asserts an empty connection decodes to no
// rows in a single request.
func TestListProjectMilestonesEmpty(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"project": map[string]any{"projectMilestones": conn([]any{}, false, "")}}
	})
	svc := newTestService(srv)

	milestones, err := svc.ListProjectMilestones(context.Background(), "project_2")
	require.NoError(t, err)
	require.Empty(t, milestones)
	require.Len(t, *calls, 1)
}

// TestListProjectMilestonesProjectNotFound asserts a null project payload
// is an error, not an empty list.
func TestListProjectMilestonesProjectNotFound(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"project": nil}
	})
	svc := newTestService(srv)

	_, err := svc.ListProjectMilestones(context.Background(), "project_999")
	require.ErrorContains(t, err, "null")
	require.Len(t, *calls, 1)
}
