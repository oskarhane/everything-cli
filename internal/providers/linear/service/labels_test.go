package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// labelNode returns a mock label node.
func labelNode(id, name, color string) map[string]any {
	return map[string]any{"id": id, "name": name, "color": color}
}

// TestListTeamLabelsFollowsCursorAcrossTwoPages covers the team-scoped
// paginated decode: labels across two pages are collected, and the second
// request follows pageInfo.endCursor while carrying the team id.
func TestListTeamLabelsFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"team": map[string]any{"labels": conn([]any{
				labelNode("label_1", "Bug", "#ff0000"),
				labelNode("label_2", "Feature", "#00ff00"),
			}, true, "cursor-1")}}
		}
		return map[string]any{"team": map[string]any{"labels": conn([]any{
			labelNode("label_3", "Chore", "#0000ff"),
		}, false, "")}}
	})
	svc := newTestService(srv)

	labels, err := svc.ListTeamLabels(context.Background(), "team_1")
	require.NoError(t, err)
	require.Equal(t, []Label{
		{ID: "label_1", Name: "Bug", Color: "#ff0000"},
		{ID: "label_2", Name: "Feature", Color: "#00ff00"},
		{ID: "label_3", Name: "Chore", Color: "#0000ff"},
	}, labels)

	// Two requests: the second follows pageInfo.endCursor, and both carry
	// the team id. The selection set is the agreed label fields, paginated
	// on the team.labels connection.
	require.Len(t, *calls, 2)
	require.Equal(t, "team_1", (*calls)[0].Variables["id"])
	require.Equal(t, "team_1", (*calls)[1].Variables["id"])
	require.NotContains(t, (*calls)[0].Variables, "after")
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	require.Equal(t, float64(pageSize), (*calls)[0].Variables["first"])
	require.Contains(t, (*calls)[0].Query, "team(id: $id)")
	require.Contains(t, (*calls)[0].Query, "labels(first: $first, after: $after)")
	require.Contains(t, (*calls)[0].Query, "nodes { id name color }")
}

func TestListTeamLabelsEmpty(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"team": map[string]any{"labels": conn([]any{}, false, "")}}
	})
	svc := newTestService(srv)

	labels, err := svc.ListTeamLabels(context.Background(), "team_1")
	require.NoError(t, err)
	require.Empty(t, labels)
	require.Len(t, *calls, 1)
}

// TestListTeamLabelsTeamNullSurfacesDigError asserts a null team payload is
// an error naming the null segment, not an empty list.
func TestListTeamLabelsTeamNullSurfacesDigError(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"team": nil}
	})
	svc := newTestService(srv)

	_, err := svc.ListTeamLabels(context.Background(), "team_999")
	require.ErrorContains(t, err, `linear response has null "team"`)
	require.Len(t, *calls, 1)
}
