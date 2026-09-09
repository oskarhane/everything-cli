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

// ptr returns a pointer to its argument.
func ptr(i service.Issue) *service.Issue { return &i }
