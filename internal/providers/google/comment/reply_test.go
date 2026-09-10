package comment

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestReplyReportsCreatedReply(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("")}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newReplyCmd, svc, "json"), "doc_1",
		"--comment", "comment_1", "--text", "Source added in v2")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, replyViewFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "reply_new", row["reply_id"])
	require.Equal(t, "2026-09-10T09:05:00Z", row["created"])
	require.Equal(t, "", row["action"])
}

func TestReplySendsSpec(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("")}
	cmdtest.RunCmd(t, newCommentLeafCmd(newReplyCmd, svc, "json"), "doc_1",
		"--comment", "comment_1", "--text", "Source added in v2")

	require.Equal(t, "doc_1", svc.replyFileID)
	require.Equal(t, "comment_1", svc.replyCommentID)
	require.Equal(t, "Source added in v2", svc.replyText)
	require.Equal(t, "", svc.replyAction, "a plain reply carries no action")
}

func TestReplyRequiresComment(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReplyCmd, svc, "json"), "doc_1", "--text", "x")

	require.ErrorContains(t, err, "--comment is required")
	require.Empty(t, svc.replyFileID, "no service call may have happened")
}

func TestReplyRequiresText(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReplyCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	require.ErrorContains(t, err, "--text is required")
	require.Empty(t, svc.replyFileID, "no service call may have happened")
}

func TestReplyPropagatesAPIError(t *testing.T) {
	svc := &fakeCommentService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReplyCmd, svc, "json"), "doc_1",
		"--comment", "comment_1", "--text", "x")

	require.ErrorIs(t, err, errAPI)
}

func TestReplyRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReplyCmd, svc, "json"),
		"--comment", "comment_1", "--text", "x")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
