package comment

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestAddReportsCreatedComment(t *testing.T) {
	svc := &fakeCommentService{created: seedCreatedComment()}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newAddCmd, svc, "json"), "doc_1", "--text", "Needs a source for this claim")

	// A single created comment reports as one JSON object: comment_id and
	// created, the same report style drive file create uses.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, commentViewFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "comment_new", row["comment_id"])
	require.Equal(t, "2026-09-10T09:00:00Z", row["created"])
}

func TestAddTableReportsCreatedComment(t *testing.T) {
	svc := &fakeCommentService{created: seedCreatedComment()}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newAddCmd, svc, "table"), "doc_1", "--text", "Needs a source")

	// go-pretty StyleLight upper-cases the headers.
	require.Contains(t, out, "COMMENT_ID")
	require.Contains(t, out, "CREATED")
	require.Contains(t, out, "comment_new")
}

func TestAddSendsSpec(t *testing.T) {
	svc := &fakeCommentService{created: seedCreatedComment()}
	cmdtest.RunCmd(t, newCommentLeafCmd(newAddCmd, svc, "json"), "doc_1", "--text", "Needs a source for this claim")

	require.Equal(t, "doc_1", svc.createFileID)
	require.Equal(t, "Needs a source for this claim", svc.createText)
}

func TestAddRequiresText(t *testing.T) {
	svc := &fakeCommentService{created: seedCreatedComment()}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newAddCmd, svc, "json"), "doc_1")

	require.ErrorContains(t, err, "--text is required")
	require.Empty(t, svc.createFileID, "no service call may have happened")
}

func TestAddPropagatesAPIError(t *testing.T) {
	svc := &fakeCommentService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newAddCmd, svc, "json"), "doc_1", "--text", "x")

	require.ErrorIs(t, err, errAPI)
}

func TestAddRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeCommentService{created: seedCreatedComment()}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newAddCmd, svc, "json"), "--text", "x")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
