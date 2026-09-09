package issue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// seedComments returns one top-level comment and one reply carrying a
// parent reference, authored by different users.
func seedComments() []service.Comment {
	return []service.Comment{
		{
			ID:        "comment_1",
			Body:      "Top-level observation",
			CreatedAt: "2026-08-01T10:00:00.000Z",
			UpdatedAt: "2026-08-01T10:00:00.000Z",
			User:      &service.NamedRef{ID: "user_1", Name: "Ada"},
		},
		{
			ID:        "comment_2",
			Body:      "Reply to the observation",
			CreatedAt: "2026-08-02T10:00:00.000Z",
			UpdatedAt: "2026-08-02T11:00:00.000Z",
			Parent:    &service.IDRef{ID: "comment_1"},
			User:      &service.NamedRef{ID: "user_2", Name: "Grace"},
		},
	}
}

func TestCommentsJSON(t *testing.T) {
	svc := &fakeService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newCommentsCmd, svc, "json"), "issue_1")

	require.Equal(t, "issue_1", svc.commentsIssueID)
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "two comments render as an array: %v", got)
	require.Len(t, arr, 2)

	top, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "comment_1", top["id"])
	require.Equal(t, "Top-level observation", top["body"])
	require.Contains(t, top, "created_at")
	require.Contains(t, top, "updated_at")
	require.NotContains(t, top, "parent_id", "top-level comments omit parent_id")
	user, ok := top["user"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user_1", user["id"])
	require.Equal(t, "Ada", user["name"])

	reply, ok := arr[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "comment_2", reply["id"])
	require.Equal(t, "comment_1", reply["parent_id"], "replies carry parent_id")
}

func TestCommentsIssueIDPassthrough(t *testing.T) {
	svc := &fakeService{comments: seedComments()}
	cmdtest.RunCmd(t, newLeafCmd(newCommentsCmd, svc, "json"), "BLA-123")

	require.Equal(t, "BLA-123", svc.commentsIssueID, "human identifiers pass through to the service")
}

func TestCommentsTable(t *testing.T) {
	svc := &fakeService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newCommentsCmd, svc, "table"), "issue_1")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "CREATED_AT")
	require.Contains(t, out, "USER")
	require.Contains(t, out, "BODY")
	require.Contains(t, out, "Top-level observation")
	require.Contains(t, out, "Grace", "the user reference flattens to its display name")
}

func TestCommentsToon(t *testing.T) {
	svc := &fakeService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newCommentsCmd, svc, "toon"), "issue_1")

	require.Contains(t, out, "body:")
	require.Contains(t, out, "Top-level observation")
}

func TestCommentsEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCommentsCmd, svc, "json"), "issue_1")

	// An empty comment list is an empty array, not an error and not null.
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "zero comments render as an empty array: %v", got)
	require.Empty(t, arr)
}

func TestCommentsError(t *testing.T) {
	svc := &fakeService{err: errors.New("issue \"BLA-999\" not found")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCommentsCmd, svc, "json"), "BLA-999")

	require.ErrorContains(t, err, `issue "BLA-999" not found`)
}
