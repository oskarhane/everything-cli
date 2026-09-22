package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSearchIssuesFollowsCursorAcrossTwoPages asserts the search term is
// sent as the required term variable, every issueField selection decodes,
// and pagination follows pageInfo.endCursor until hasNextPage is false.
func TestSearchIssuesFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"searchIssues": conn([]any{
				issueNode("issue_1", "ENG-1", "Login redirect broken"),
				issueNode("issue_2", "ENG-2", "Redirect loop on logout"),
			}, true, "cursor-1")}
		}
		return map[string]any{"searchIssues": conn([]any{
			issueNode("issue_3", "ENG-3", "Redirect after signup"),
		}, false, "")}
	})
	svc := newTestService(srv)

	issues, err := svc.SearchIssues(context.Background(), "redirect")
	require.NoError(t, err)
	require.Len(t, issues, 3)
	require.Equal(t, "ENG-1", issues[0].Identifier)
	require.Equal(t, "ENG-3", issues[2].Identifier)
	require.Equal(t, "In Progress", issues[0].State.Name)
	require.Equal(t, "ENG", issues[0].Team.Key)

	// Two requests: the term variable rides on both, the second follows
	// pageInfo.endCursor, and the query is the searchIssues connection
	// selecting the shared issue fields.
	require.Len(t, *calls, 2)
	require.Equal(t, "redirect", (*calls)[0].Variables["term"])
	require.Equal(t, "redirect", (*calls)[1].Variables["term"])
	require.NotContains(t, (*calls)[0].Variables, "after")
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	require.Equal(t, float64(pageSize), (*calls)[0].Variables["first"])
	require.Contains(t, (*calls)[0].Query, "searchIssues(term: $term, first: $first, after: $after)")
	require.Contains(t, (*calls)[0].Query, "nodes { "+issueFields+" }")
}

// TestSearchIssuesEmptyResult asserts a no-match search decodes to an empty
// (nil) issue list from a single request — no pagination, no error.
func TestSearchIssuesEmptyResult(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"searchIssues": conn([]any{}, false, "")}
	})
	svc := newTestService(srv)

	issues, err := svc.SearchIssues(context.Background(), "no-such-thing")
	require.NoError(t, err)
	require.Empty(t, issues)
	require.Len(t, *calls, 1)
}
