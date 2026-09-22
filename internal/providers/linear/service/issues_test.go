package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// updateInput extracts the GraphQL input map of the single recorded call.
func updateInput(t *testing.T, calls *[]gqlCall) map[string]any {
	t.Helper()
	require.Len(t, *calls, 1)
	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	return input
}

// runUpdate performs one UpdateIssue against a mock that always succeeds
// and returns the recorded input map.
func runUpdate(t *testing.T, in UpdateIssueInput) map[string]any {
	t.Helper()
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueUpdate": map[string]any{
			"success": true, "issue": issueNode("issue_1", "ENG-1", "X"),
		}}
	})
	svc := newTestService(srv)

	_, err := svc.UpdateIssue(context.Background(), "ENG-1", in)
	require.NoError(t, err)
	return updateInput(t, calls)
}

func TestCreateIssueMapsNewFields(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueCreate": map[string]any{
			"success": true, "issue": issueNode("issue_new", "ENG-4", "Child"),
		}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateIssue(context.Background(), CreateIssueInput{
		TeamID:             "team_1",
		Title:              "Child",
		ParentID:           "issue_parent",
		LabelIDs:           []string{"label_1", "label_2"},
		Priority:           2,
		DueDate:            "2026-10-01",
		Estimate:           3,
		CycleID:            "cycle_1",
		ProjectMilestoneID: "milestone_1",
	})
	require.NoError(t, err)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "issue_parent", input["parentId"])
	// The input map round-trips through JSON, so numbers decode as float64.
	require.Equal(t, []any{"label_1", "label_2"}, input["labelIds"])
	require.Equal(t, float64(2), input["priority"])
	require.Equal(t, "2026-10-01", input["dueDate"])
	require.Equal(t, float64(3), input["estimate"])
	require.Equal(t, "cycle_1", input["cycleId"])
	require.Equal(t, "milestone_1", input["projectMilestoneId"])
}

func TestCreateIssueOmitsEmptyNewFields(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueCreate": map[string]any{
			"success": true, "issue": issueNode("issue_new", "ENG-4", "Lean"),
		}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateIssue(context.Background(), CreateIssueInput{TeamID: "team_1", Title: "Lean"})
	require.NoError(t, err)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{"parentId", "labelIds", "priority", "dueDate", "estimate", "cycleId", "projectMilestoneId"} {
		require.NotContains(t, input, key)
	}
}

// TestUpdateIssueNewFieldMapping asserts each new pointer field maps to its
// GraphQL key when set, and is omitted when nil.
func TestUpdateIssueNewFieldMapping(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(n int) *int { return &n }

	t.Run("all set", func(t *testing.T) {
		input := runUpdate(t, UpdateIssueInput{
			ParentID:           strPtr("issue_parent"),
			LabelIDs:           &[]string{"label_1"},
			Priority:           intPtr(1),
			DueDate:            strPtr("2026-12-24"),
			Estimate:           intPtr(5),
			CycleID:            strPtr("cycle_1"),
			ProjectMilestoneID: strPtr("milestone_1"),
		})
		require.Equal(t, "issue_parent", input["parentId"])
		require.Equal(t, []any{"label_1"}, input["labelIds"])
		require.Equal(t, float64(1), input["priority"])
		require.Equal(t, "2026-12-24", input["dueDate"])
		require.Equal(t, float64(5), input["estimate"])
		require.Equal(t, "cycle_1", input["cycleId"])
		require.Equal(t, "milestone_1", input["projectMilestoneId"])
	})

	t.Run("nil pointers omit the keys", func(t *testing.T) {
		input := runUpdate(t, UpdateIssueInput{Title: "Only title"})
		require.Equal(t, map[string]any{"title": "Only title"}, input)
	})
}

// TestUpdateIssueClearSemantics asserts the omit-vs-clear contract: a nil
// pointer omits the key, a pointer to the empty value sends the API's clear
// signal (null for scalars, [] for labels).
func TestUpdateIssueClearSemantics(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("empty parentId sends null to un-parent", func(t *testing.T) {
		input := runUpdate(t, UpdateIssueInput{ParentID: strPtr("")})
		require.Contains(t, input, "parentId")
		require.Nil(t, input["parentId"])
	})

	t.Run("empty labelIds sends [] to remove all labels", func(t *testing.T) {
		input := runUpdate(t, UpdateIssueInput{LabelIDs: &[]string{}})
		require.Equal(t, []any{}, input["labelIds"])
	})

	t.Run("nil labelIds slice pointer still sends []", func(t *testing.T) {
		var ids []string
		input := runUpdate(t, UpdateIssueInput{LabelIDs: &ids})
		require.Equal(t, []any{}, input["labelIds"])
	})

	t.Run("empty dueDate sends null to clear the due date", func(t *testing.T) {
		input := runUpdate(t, UpdateIssueInput{DueDate: strPtr("")})
		require.Contains(t, input, "dueDate")
		require.Nil(t, input["dueDate"])
	})

	t.Run("empty cycleId and projectMilestoneId send null", func(t *testing.T) {
		input := runUpdate(t, UpdateIssueInput{CycleID: strPtr(""), ProjectMilestoneID: strPtr("")})
		require.Contains(t, input, "cycleId")
		require.Nil(t, input["cycleId"])
		require.Contains(t, input, "projectMilestoneId")
		require.Nil(t, input["projectMilestoneId"])
	})
}

// TestGetIssueSelectsDetailFields asserts GetIssue selects the detail field
// set and decodes the detail-only wire fields.
func TestGetIssueSelectsDetailFields(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		node := issueNode("issue_1", "ENG-1", "Detailed")
		node["parent"] = map[string]any{"id": "issue_0", "identifier": "ENG-0", "title": "Epic"}
		node["children"] = conn([]any{
			map[string]any{"id": "issue_2", "identifier": "ENG-2", "title": "Sub task"},
		}, false, "")
		node["priority"] = 2
		node["priorityLabel"] = "High"
		node["dueDate"] = "2026-10-01"
		node["estimate"] = 3
		node["cycle"] = map[string]any{"id": "cycle_1", "name": "Cycle 12"}
		node["projectMilestone"] = map[string]any{"id": "milestone_1", "name": "Beta"}
		return map[string]any{"issue": node}
	})
	svc := newTestService(srv)

	issue, err := svc.GetIssue(context.Background(), "ENG-1")
	require.NoError(t, err)

	query := (*calls)[0].Query
	require.Contains(t, query, "parent { id identifier title }")
	require.Contains(t, query, "children { nodes { id identifier title } }")
	require.Contains(t, query, "priority priorityLabel dueDate estimate")
	require.Contains(t, query, "cycle { id name } projectMilestone { id name }")

	require.Equal(t, "ENG-0", issue.Parent.Identifier)
	require.Len(t, issue.Children, 1)
	require.Equal(t, "Sub task", issue.Children[0].Title)
	require.Equal(t, 2, issue.Priority)
	require.Equal(t, "High", issue.PriorityLabel)
	require.Equal(t, "2026-10-01", issue.DueDate)
	require.Equal(t, 3, *issue.Estimate)
	require.Equal(t, "Cycle 12", issue.Cycle.Name)
	require.Equal(t, "Beta", issue.Milestone.Name)
}

// TestLeanFieldSetsKeepParentOnly asserts the shared issueFields selection
// gained parent but stays lean: list and mutation payloads must not select
// the detail-only fields.
func TestLeanFieldSetsKeepParentOnly(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issues": conn([]any{
			issueNode("issue_1", "ENG-1", "First"),
		}, false, "")}
	})
	svc := newTestService(srv)

	_, err := svc.ListIssues(context.Background(), IssueFilter{})
	require.NoError(t, err)

	query := (*calls)[0].Query
	require.Contains(t, query, "parent { id identifier title }")
	require.NotContains(t, query, "children")
	require.NotContains(t, query, "priorityLabel")
	require.NotContains(t, query, "projectMilestone")
}
