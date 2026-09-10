package comment

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSONDefaultsToUnresolved(t *testing.T) {
	svc := &fakeCommentService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newListCmd, svc, "json"), "doc_1")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2, "resolved comment_3 must be hidden by default")

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, listFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "comment_1", first["comment_id"])
	require.Equal(t, "Oskar Hane", first["author"])
	require.Equal(t, "2026-09-08T09:15:00Z", first["created"])
	require.Equal(t, false, first["resolved"])
	require.Equal(t, "revenue is up 12%", first["quoted"])
	require.Equal(t, "Needs a source for this claim", first["content"])

	// comment_2 has no author and no quoted region: both render as "".
	second := rows[1].(map[string]any)
	require.Equal(t, "", second["author"])
	require.Equal(t, "", second["quoted"])
	require.Empty(t, second["replies"], "a nil reply list renders as an empty array")
}

func TestListAllIncludesResolved(t *testing.T) {
	svc := &fakeCommentService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newListCmd, svc, "json"), "doc_1", "--all")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
	ids := []string{
		rows[0].(map[string]any)["comment_id"].(string),
		rows[1].(map[string]any)["comment_id"].(string),
		rows[2].(map[string]any)["comment_id"].(string),
	}
	require.ElementsMatch(t, []string{"comment_1", "comment_2", "comment_3"}, ids)
	require.Equal(t, true, rows[2].(map[string]any)["resolved"])
}

// TestListJSONNestsReplies pins the JSON replies shape: an array of
// {reply_id, author, created, action, content} objects per comment row.
func TestListJSONNestsReplies(t *testing.T) {
	svc := &fakeCommentService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newListCmd, svc, "json"), "doc_1", "--all")

	rows := cmdtest.DecodeJSON(t, out).([]any)
	first := rows[0].(map[string]any)
	replies, ok := first["replies"].([]any)
	require.True(t, ok, "replies must be a JSON array, got: %v", first["replies"])
	require.Len(t, replies, 2)

	r1, ok := replies[0].(map[string]any)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"reply_id", "author", "created", "action", "content"}, cmdtest.JSONKeys(t, r1))
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, r1))
	require.Equal(t, "reply_1", r1["reply_id"])
	require.Equal(t, "Ada Lovelace", r1["author"])
	require.Equal(t, "2026-09-08T10:00:00Z", r1["created"])
	require.Equal(t, "", r1["action"])
	require.Equal(t, "Source added in v2", r1["content"])

	r2 := replies[1].(map[string]any)
	require.Equal(t, "resolve", r2["action"])
	require.Equal(t, "", r2["content"])
}

func TestListTableCompactsRepliesToCount(t *testing.T) {
	svc := &fakeCommentService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newCommentLeafCmd(newListCmd, svc, "table"), "doc_1", "--all")

	// go-pretty StyleLight upper-cases the headers.
	for _, header := range []string{"COMMENT_ID", "AUTHOR", "CREATED", "RESOLVED", "QUOTED", "CONTENT", "REPLIES"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "Needs a source for this claim")
	// The table cell is the reply COUNT, not the nested objects: no reply
	// text may leak into a table row.
	require.NotContains(t, out, "Source added in v2")
}

// TestListEmptyRendersZeroRows covers the seam's nil-slice contract: a file
// with no comments renders as zero rows in every format, never panicking.
func TestListEmptyRendersZeroRows(t *testing.T) {
	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			svc := &fakeCommentService{comments: nil}
			out := cmdtest.RunCmd(t, newCommentLeafCmd(newListCmd, svc, format), "doc_1")

			require.NotEmpty(t, out)
			if format == "json" {
				require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
			}
		})
	}
}

func TestListPassesFileID(t *testing.T) {
	svc := &fakeCommentService{comments: seedComments()}
	cmdtest.RunCmd(t, newCommentLeafCmd(newListCmd, svc, "json"), "doc_1")

	require.Equal(t, "doc_1", svc.listID)
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeCommentService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newListCmd, svc, "json"), "doc_1")

	require.ErrorIs(t, err, errAPI)
}

func TestListRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeCommentService{}
	_, err := cmdtest.RunCmdErr(t, newCommentLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
