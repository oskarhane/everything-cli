package comment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateRequiresBody(t *testing.T) {
	svc := &fakeService{created: seedComment()}
	_, err := cmdtest.RunCmdErr(t, createCmd(svc, "json"), "issue_1")

	require.Error(t, err, "cobra rejects a missing --body before RunE")
	require.Empty(t, svc.createIssueID, "the service is never dialed without a body")
}

func TestCreatePassesIssueAndFlags(t *testing.T) {
	svc := &fakeService{created: seedComment()}
	cmdtest.RunCmd(t, createCmd(svc, "json"), "BLA-123", "--body", "Looks good to me")

	require.Equal(t, "BLA-123", svc.createIssueID, "the positional id passes through")
	require.Equal(t, "Looks good to me", svc.createInput.Body)
	require.Empty(t, svc.createInput.ParentID, "omitting --parent creates a top-level comment")
}

func TestCreatePassesParent(t *testing.T) {
	svc := &fakeService{created: seedComment()}
	cmdtest.RunCmd(t, createCmd(svc, "json"), "issue_1", "--body", "Agreed", "--parent", "comment_0")

	require.Equal(t, "comment_0", svc.createInput.ParentID)
	require.Equal(t, "Agreed", svc.createInput.Body)
}

func TestCreateJSONIsOneObject(t *testing.T) {
	svc := &fakeService{created: seedComment()}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"), "issue_1", "--body", "Looks good to me")

	got := cmdtest.DecodeJSON(t, out)
	m, ok := got.(map[string]any)
	require.True(t, ok, "one created comment renders as a single object: %v", got)
	require.Equal(t, "comment_1", m["id"])
	require.Equal(t, "Looks good to me", m["body"])
	require.Contains(t, m, "created_at")
	require.Contains(t, m, "updated_at")
	require.NotContains(t, m, "parent_id", "top-level comments omit parent_id")
	user, ok := m["user"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user_1", user["id"])
	require.Equal(t, "Ada", user["name"])
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, m))
}

func TestCreateJSONParentID(t *testing.T) {
	reply := seedComment()
	reply.Parent = &service.IDRef{ID: "comment_0"}
	svc := &fakeService{created: reply}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"), "issue_1", "--body", "Agreed")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "comment_0", m["parent_id"], "replies carry parent_id")
}

func TestCreateJSONOmitsNilUser(t *testing.T) {
	created := seedComment()
	created.User = nil
	svc := &fakeService{created: created}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"), "issue_1", "--body", "Looks good to me")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.NotContains(t, m, "user", "a nil author is omitted")
}

func TestCreateTable(t *testing.T) {
	svc := &fakeService{created: seedComment()}
	out := cmdtest.RunCmd(t, createCmd(svc, "table"), "issue_1", "--body", "Looks good to me")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "CREATED_AT")
	require.Contains(t, out, "USER")
	require.Contains(t, out, "BODY")
	require.Contains(t, out, "Ada", "the author reference flattens to its display name")
	require.Contains(t, out, "Looks good to me")
}

func TestCreateToon(t *testing.T) {
	svc := &fakeService{created: seedComment()}
	out := cmdtest.RunCmd(t, createCmd(svc, "toon"), "issue_1", "--body", "Looks good to me")

	require.Contains(t, out, "body:")
	require.Contains(t, out, "Looks good to me")
}

func TestCreateError(t *testing.T) {
	svc := &fakeService{err: errors.New("issue \"BLA-999\" not found")}
	_, err := cmdtest.RunCmdErr(t, createCmd(svc, "json"), "BLA-999", "--body", "hi")

	require.ErrorContains(t, err, `issue "BLA-999" not found`)
}
