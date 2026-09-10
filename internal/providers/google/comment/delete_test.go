package comment

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeCommentService{}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--comment", "comment_1")

	// The refusal wording is contractual: it names the comment, the file,
	// and the --force remedy, and no service call may have happened. A leaf
	// with no parent chain falls back to the provider-neutral resolve hint.
	require.ErrorContains(t, err, `refusing to delete comment "comment_1" on file "doc_1" without --force`)
	require.ErrorContains(t, err, "cannot be undone")
	require.ErrorContains(t, err, `"comment resolve <file-id> --comment <id>"`)
	require.NotContains(t, err.Error(), "google docs")
	require.Zero(t, svc.deleteCalls)
}

// TestDeleteRefusalNamesTheInvokedPath hangs the delete leaf under a
// synthetic root -> google -> <resource> -> comment chain for each real
// parent, so the refusal's resolve hint names the path actually invoked.
func TestDeleteRefusalNamesTheInvokedPath(t *testing.T) {
	for _, resource := range []string{"docs", "slides"} {
		t.Run(resource, func(t *testing.T) {
			svc := &fakeCommentService{}
			deleteCmd := newCommentLeafCmd(newDeleteCmd, svc, "json")
			commentParent := &cobra.Command{Use: "comment"}
			commentParent.AddCommand(deleteCmd)
			resourceParent := &cobra.Command{Use: resource}
			resourceParent.AddCommand(commentParent)
			google := &cobra.Command{Use: "google"}
			google.AddCommand(resourceParent)
			root := &cobra.Command{Use: "everything-cli"}
			root.AddCommand(google)

			_, err := cmdtest.RunCmdErr(t, root,
				"google", resource, "comment", "delete", "doc_1", "--comment", "comment_1")

			require.ErrorContains(t, err, "without --force")
			require.ErrorContains(t, err,
				`"everything-cli google `+resource+` comment resolve <file-id> --comment <id>"`)
			require.Zero(t, svc.deleteCalls)
		})
	}
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
