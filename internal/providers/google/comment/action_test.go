package comment

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// actionLeaves drives one table over the two action-reply leaves: they share
// newActionCmd and differ only in the action word.
var actionLeaves = []struct {
	name  string
	build func(*app.Config, service.Dialer[service.CommentService]) *cobra.Command
}{
	{"resolve", newResolveCmd},
	{"reopen", newReopenCmd},
}

func TestActionSendsActionWithoutContent(t *testing.T) {
	for _, leaf := range actionLeaves {
		t.Run(leaf.name, func(t *testing.T) {
			svc := &fakeCommentService{replied: seedReplied(leaf.name)}
			cmdtest.RunCmd(t, newCommentLeafCmd(leaf.build, svc, "json"), "doc_1", "--comment", "comment_1")

			require.Equal(t, "doc_1", svc.replyFileID)
			require.Equal(t, "comment_1", svc.replyCommentID)
			require.Equal(t, "", svc.replyText, "an action reply carries no content")
			require.Equal(t, leaf.name, svc.replyAction)
		})
	}
}

func TestActionReportsCreatedReply(t *testing.T) {
	for _, leaf := range actionLeaves {
		t.Run(leaf.name, func(t *testing.T) {
			svc := &fakeCommentService{replied: seedReplied(leaf.name)}
			out := cmdtest.RunCmd(t, newCommentLeafCmd(leaf.build, svc, "json"), "doc_1", "--comment", "comment_1")

			row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
			require.True(t, ok, "expected a JSON object, got: %s", out)
			require.Equal(t, "reply_new", row["reply_id"])
			require.Equal(t, leaf.name, row["action"])
		})
	}
}

func TestActionRequiresComment(t *testing.T) {
	for _, leaf := range actionLeaves {
		t.Run(leaf.name, func(t *testing.T) {
			svc := &fakeCommentService{replied: seedReplied(leaf.name)}
			_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(leaf.build, svc, "json"), "doc_1")

			require.ErrorContains(t, err, "--comment is required")
			require.Empty(t, svc.replyFileID, "no service call may have happened")
		})
	}
}

func TestActionPropagatesAPIError(t *testing.T) {
	for _, leaf := range actionLeaves {
		t.Run(leaf.name, func(t *testing.T) {
			svc := &fakeCommentService{err: errAPI}
			_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(leaf.build, svc, "json"), "doc_1", "--comment", "comment_1")

			require.ErrorIs(t, err, errAPI)
		})
	}
}

func TestActionRequiresExactlyOneArg(t *testing.T) {
	for _, leaf := range actionLeaves {
		t.Run(leaf.name, func(t *testing.T) {
			svc := &fakeCommentService{replied: seedReplied(leaf.name)}
			_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(leaf.build, svc, "json"), "--comment", "comment_1")

			require.Contains(t, err.Error(), "accepts 1 arg")
		})
	}
}
