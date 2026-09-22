package issue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// TestToViewNewFields checks toView maps state type, creator, and the
// transition timestamps onto the rendered shape.
func TestToViewNewFields(t *testing.T) {
	cases := []struct {
		name    string
		issue   service.Issue
		state   *stateView
		creator *refView
		started string
	}{
		{
			name:    "full seed",
			issue:   seedIssue(),
			state:   &stateView{ID: "state_1", Name: "In Progress", Type: "started"},
			creator: &refView{ID: "user_2", Name: "Grace"},
			started: "2026-08-01T11:00:00.000Z",
		},
		{
			name:  "sparse issue",
			issue: service.Issue{ID: "issue_2", Identifier: "ENG-2", Title: "Sparse"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := toView(&tc.issue)
			require.Equal(t, tc.state, v.State)
			require.Equal(t, tc.creator, v.Creator)
			assert.Equal(t, tc.started, v.StartedAt)
			assert.Empty(t, v.CompletedAt)
			assert.Empty(t, v.CanceledAt)
		})
	}
}

// TestJSONRowNewFields checks the JSON/TOON row: state carries type, creator
// and empty timestamps are omitted entirely rather than rendered as null or
// empty strings.
func TestJSONRowNewFields(t *testing.T) {
	t.Run("full seed", func(t *testing.T) {
		row := jsonRow(toView(ptr(seedIssue())))
		require.Contains(t, row, "creator")
		require.Equal(t, map[string]any{"id": "user_2", "name": "Grace"}, row["creator"])
		require.Equal(t, "2026-08-01T11:00:00.000Z", row["started_at"])
		assert.NotContains(t, row, "completed_at")
		assert.NotContains(t, row, "canceled_at")

		state, ok := row["state"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, map[string]any{"id": "state_1", "name": "In Progress", "type": "started"}, state)
	})

	t.Run("sparse issue", func(t *testing.T) {
		row := jsonRow(toView(&service.Issue{ID: "issue_2", Identifier: "ENG-2", Title: "Sparse"}))
		assert.Nil(t, row["state"])
		assert.NotContains(t, row, "creator")
		assert.NotContains(t, row, "started_at")
		assert.NotContains(t, row, "completed_at")
		assert.NotContains(t, row, "canceled_at")
	})

	t.Run("completed timestamps appear", func(t *testing.T) {
		i := seedIssue()
		i.CompletedAt = "2026-08-03T10:00:00.000Z"
		i.CanceledAt = "2026-08-04T10:00:00.000Z"
		row := jsonRow(toView(&i))
		assert.Equal(t, "2026-08-03T10:00:00.000Z", row["completed_at"])
		assert.Equal(t, "2026-08-04T10:00:00.000Z", row["canceled_at"])
	})
}

// TestTableRowNewFields checks the flattened table row carries the new
// cells, with empties staying empty.
func TestTableRowNewFields(t *testing.T) {
	t.Run("full seed", func(t *testing.T) {
		row := tableRow(toView(ptr(seedIssue())))
		assert.Equal(t, "Grace", row["creator"])
		assert.Equal(t, "2026-08-01T11:00:00.000Z", row["started_at"])
		assert.Empty(t, row["completed_at"])
		assert.Empty(t, row["canceled_at"])
		assert.Equal(t, "In Progress", row["state"])
	})

	t.Run("sparse issue", func(t *testing.T) {
		row := tableRow(toView(&service.Issue{ID: "issue_2", Identifier: "ENG-2", Title: "Sparse"}))
		assert.Empty(t, row["creator"])
		assert.Empty(t, row["started_at"])
		assert.Empty(t, row["completed_at"])
		assert.Empty(t, row["canceled_at"])
		assert.Empty(t, row["state"])
	})
}

// TestFieldSlices pins the table column lists: listFields byte-identical,
// detailFields carrying the four new names.
func TestFieldSlices(t *testing.T) {
	assert.Equal(t, []string{"identifier", "title", "state", "assignee", "team", "updated_at"}, listFields)

	for _, name := range []string{"creator", "started_at", "completed_at", "canceled_at"} {
		assert.Contains(t, detailFields, name)
	}
}

// detailIssue returns seedIssue with every detail-only field populated.
func detailIssue() service.Issue {
	i := seedIssue()
	i.Parent = &service.IssueRef{ID: "issue_0", Identifier: "ENG-0", Title: "Parent epic"}
	i.Children = []service.IssueRef{
		{ID: "issue_2", Identifier: "ENG-2", Title: "Child one"},
		{ID: "issue_3", Identifier: "ENG-3", Title: "Child two"},
	}
	i.Priority = 2
	i.PriorityLabel = "High"
	i.DueDate = "2026-09-30"
	i.Estimate = ptrInt(3)
	i.Cycle = &service.NamedRef{ID: "cycle_1", Name: "Cycle 12"}
	i.Milestone = &service.NamedRef{ID: "ms_1", Name: "v2.0"}
	return i
}

// ptrInt returns a pointer to its argument.
func ptrInt(n int) *int { return &n }

// TestToViewDetailFields checks toView maps the detail-only wire fields
// (parent, children, priority, due date, estimate, cycle, milestone) onto
// the rendered shape, leaving them absent on sparse issues.
func TestToViewDetailFields(t *testing.T) {
	t.Run("full detail", func(t *testing.T) {
		v := toView(ptr(detailIssue()))
		require.Equal(t, &issueRefView{ID: "issue_0", Identifier: "ENG-0", Title: "Parent epic"}, v.Parent)
		require.Equal(t, []issueRefView{
			{ID: "issue_2", Identifier: "ENG-2", Title: "Child one"},
			{ID: "issue_3", Identifier: "ENG-3", Title: "Child two"},
		}, v.Children)
		assert.Equal(t, 2, v.Priority)
		assert.Equal(t, "High", v.PriorityLabel)
		assert.Equal(t, "2026-09-30", v.DueDate)
		require.NotNil(t, v.Estimate)
		assert.Equal(t, 3, *v.Estimate)
		require.Equal(t, &refView{ID: "cycle_1", Name: "Cycle 12"}, v.Cycle)
		require.Equal(t, &refView{ID: "ms_1", Name: "v2.0"}, v.Milestone)
	})

	t.Run("sparse issue", func(t *testing.T) {
		v := toView(&service.Issue{ID: "issue_2", Identifier: "ENG-2", Title: "Sparse"})
		assert.Nil(t, v.Parent)
		assert.Empty(t, v.Children)
		assert.Zero(t, v.Priority)
		assert.Empty(t, v.PriorityLabel)
		assert.Empty(t, v.DueDate)
		assert.Nil(t, v.Estimate)
		assert.Nil(t, v.Cycle)
		assert.Nil(t, v.Milestone)
	})
}

// TestJSONRowDetailFields checks the JSON/TOON row carries the detail-only
// fields when present and omits them entirely when absent.
func TestJSONRowDetailFields(t *testing.T) {
	t.Run("full detail", func(t *testing.T) {
		row := jsonRow(toView(ptr(detailIssue())))
		assert.Equal(t, map[string]any{"id": "issue_0", "identifier": "ENG-0", "title": "Parent epic"}, row["parent"])
		assert.Equal(t, []map[string]any{
			{"id": "issue_2", "identifier": "ENG-2", "title": "Child one"},
			{"id": "issue_3", "identifier": "ENG-3", "title": "Child two"},
		}, row["children"])
		assert.Equal(t, 2, row["priority"])
		assert.Equal(t, "High", row["priority_label"])
		assert.Equal(t, "2026-09-30", row["due_date"])
		assert.Equal(t, 3, row["estimate"])
		assert.Equal(t, map[string]any{"id": "cycle_1", "name": "Cycle 12"}, row["cycle"])
		assert.Equal(t, map[string]any{"id": "ms_1", "name": "v2.0"}, row["milestone"])
	})

	t.Run("sparse issue", func(t *testing.T) {
		row := jsonRow(toView(&service.Issue{ID: "issue_2", Identifier: "ENG-2", Title: "Sparse"}))
		assert.Equal(t, 0, row["priority"])
		assert.NotContains(t, row, "priority_label")
		assert.NotContains(t, row, "due_date")
		assert.NotContains(t, row, "estimate")
		assert.NotContains(t, row, "parent")
		assert.NotContains(t, row, "children")
		assert.NotContains(t, row, "cycle")
		assert.NotContains(t, row, "milestone")
	})
}

// TestTableRowDetailFields checks the flattened table row: parent renders as
// an identifier, children as a comma-separated identifier list, and the
// remaining detail fields as plain cells.
func TestTableRowDetailFields(t *testing.T) {
	t.Run("full detail", func(t *testing.T) {
		row := tableRow(toView(ptr(detailIssue())))
		assert.Equal(t, "ENG-0", row["parent"])
		assert.Equal(t, "ENG-2, ENG-3", row["children"])
		assert.Equal(t, "High", row["priority"])
		assert.Equal(t, "2026-09-30", row["due_date"])
		assert.Equal(t, "3", row["estimate"])
		assert.Equal(t, "Cycle 12", row["cycle"])
		assert.Equal(t, "v2.0", row["milestone"])
	})

	t.Run("priority falls back to number", func(t *testing.T) {
		i := seedIssue()
		i.Priority = 1
		row := tableRow(toView(&i))
		assert.Equal(t, "1", row["priority"])
	})

	t.Run("sparse issue", func(t *testing.T) {
		row := tableRow(toView(&service.Issue{ID: "issue_2", Identifier: "ENG-2", Title: "Sparse"}))
		assert.Empty(t, row["parent"])
		assert.Empty(t, row["children"])
		assert.Empty(t, row["priority"])
		assert.Empty(t, row["due_date"])
		assert.Empty(t, row["estimate"])
		assert.Empty(t, row["cycle"])
		assert.Empty(t, row["milestone"])
	})
}

// TestDetailFieldSlices pins the detail-only table columns.
func TestDetailFieldSlices(t *testing.T) {
	for _, name := range []string{"parent", "children", "priority", "due_date", "estimate", "cycle", "milestone"} {
		assert.Contains(t, detailFields, name)
	}
}

// ptr returns a pointer to its argument.
func ptr(i service.Issue) *service.Issue { return &i }
