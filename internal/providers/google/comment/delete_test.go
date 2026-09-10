package comment

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeCommentService{}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	// The refusal wording is contractual: it names the comment, the file,
	// and the --force remedy, and no service call may have happened.
	require.ErrorContains(t, err, `refusing to delete comment "comment_1" on file "doc_1" without --force`)
	require.ErrorContains(t, err, "cannot be undone")
	require.Zero(t, svc.deleteCalls)
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeCommentService{}
	cmdtest.RunCmd(t, newCommentLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--comment", "comment_1", "--force")

	require.Equal(t, 1, svc.deleteCalls)
	require.Equal(t, "doc_1", svc.deleteFileID)
	require.Equal(t, "comment_1", svc.deleteCommentID)
}

func TestDeleteRequiresComment(t *testing.T) {
	svc := &fakeCommentService{}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--force")

	require.ErrorContains(t, err, "--comment is required")
	require.Zero(t, svc.deleteCalls, "no service call may have happened")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeCommentService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newDeleteCmd, svc, "json"), "doc_1",
		"--comment", "comment_1", "--force")

	require.ErrorIs(t, err, errAPI)
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeCommentService{}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newDeleteCmd, svc, "json"), "--comment", "comment_1", "--force")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
