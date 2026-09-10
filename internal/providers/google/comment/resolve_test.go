package comment

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestResolveSendsActionWithoutContent(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("resolve")}
	cmdtest.RunCmd(t, newCommentLeafCmd(newResolveCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	require.Equal(t, "doc_1", svc.replyFileID)
	require.Equal(t, "comment_1", svc.replyCommentID)
	require.Equal(t, "", svc.replyText, "a resolve reply carries no content")
	require.Equal(t, "resolve", svc.replyAction)
}

func TestResolveReportsCreatedReply(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("resolve")}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newResolveCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	require.Equal(t, "reply_new", row["reply_id"])
	require.Equal(t, "resolve", row["action"])
}

func TestResolveRequiresComment(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("resolve")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newResolveCmd, svc, "json"), "doc_1")

	require.ErrorContains(t, err, "--comment is required")
	require.Empty(t, svc.replyFileID, "no service call may have happened")
}

func TestResolvePropagatesAPIError(t *testing.T) {
	svc := &fakeCommentService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newResolveCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	require.ErrorIs(t, err, errAPI)
}

func TestResolveRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeCommentService{replied: seedReplied("resolve")}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newResolveCmd, svc, "json"), "--comment", "comment_1")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
