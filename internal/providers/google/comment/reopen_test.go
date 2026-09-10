package comment

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestReopenSendsActionWithoutContent(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("reopen")}
	cmdtest.RunCmd(t, newCommentLeafCmd(newReopenCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	require.Equal(t, "doc_1", svc.replyFileID)
	require.Equal(t, "comment_1", svc.replyCommentID)
	require.Equal(t, "", svc.replyText, "a reopen reply carries no content")
	require.Equal(t, "reopen", svc.replyAction)
}

func TestReopenReportsCreatedReply(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("reopen")}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newReopenCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	require.Equal(t, "reply_new", row["reply_id"])
	require.Equal(t, "reopen", row["action"])
}

func TestReopenRequiresComment(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("reopen")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReopenCmd, svc, "json"), "doc_1")

	require.ErrorContains(t, err, "--comment is required")
	require.Empty(t, svc.replyFileID, "no service call may have happened")
}

func TestReopenPropagatesAPIError(t *testing.T) {
	svc := &fakeCommentService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReopenCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	require.ErrorIs(t, err, errAPI)
}

func TestReopenRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("reopen")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newReopenCmd, svc, "json"), "--comment", "comment_1")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
