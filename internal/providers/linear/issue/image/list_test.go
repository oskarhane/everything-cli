package image

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSON(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, &fakeCommentService{}, &fakeFileService{}, "json"), "issue_1")

	require.Equal(t, "issue_1", issue.gotID)
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "two images render as an array: %v", got)
	require.Len(t, arr, 2)

	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000001", first["url"])
	require.Equal(t, "login-form.png", first["filename"])
	require.Equal(t, "description", first["source"])
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, first))

	second, ok := arr[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "graph", second["filename"], "a missing extension is only fixed on download, from the response")
}

func TestListSingleImageRendersAsOneObject(t *testing.T) {
	issue := seedIssue()
	issue.Description = "![only](https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000009)"
	svc := &fakeIssueService{issue: issue}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, &fakeCommentService{}, &fakeFileService{}, "json"), "issue_1")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one image renders as a single object: %v", out)
	require.Equal(t, "only", m["filename"])
}

func TestListIssueIDPassthrough(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, &fakeCommentService{}, &fakeFileService{}, "json"), "BLA-123")

	require.Equal(t, "BLA-123", issue.gotID, "human identifiers pass through to the service")
}

func TestListComments(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	comment := &fakeCommentService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, comment, &fakeFileService{}, "json"), "issue_1", "--comments")

	require.Equal(t, "issue_1", comment.gotID)
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "three images render as an array: %v", got)
	require.Len(t, arr, 3)
	// Description first, then the fresh comment image; the repeated URL was
	// deduplicated away.
	third, ok := arr[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000003", third["url"])
	require.Equal(t, "fresh.png", third["filename"])
	require.Equal(t, "comment", third["source"])
}

func TestListDedupAcrossSources(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	comment := &fakeCommentService{comments: seedComments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, comment, &fakeFileService{}, "json"), "issue_1", "--comments")

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	var urls []string
	for _, row := range arr {
		m, ok := row.(map[string]any)
		require.True(t, ok)
		urls = append(urls, m["url"].(string))
	}
	// u2 appears in the description and a comment but renders once, as the
	// description's.
	require.Equal(t, []string{
		"https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000001",
		"https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000002",
		"https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000003",
	}, urls)
	for _, row := range arr {
		m := row.(map[string]any)
		if m["url"] == "https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000002" {
			require.Equal(t, "description", m["source"], "the first occurrence wins the source label")
			require.Equal(t, "graph", m["filename"], "the first occurrence wins the alt text")
		}
	}
}

func TestListDoesNotScanCommentsByDefault(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	comment := &fakeCommentService{comments: seedComments()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, comment, &fakeFileService{}, "json"), "issue_1")

	require.False(t, comment.dialed, "without --comments the comment service is never dialed")
}

func TestListTable(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, &fakeCommentService{}, &fakeFileService{}, "table"), "issue_1")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "FILENAME")
	require.Contains(t, out, "SOURCE")
	require.Contains(t, out, "URL")
	require.Contains(t, out, "login-form.png")
	require.Contains(t, out, "description")
}

func TestListToon(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, &fakeCommentService{}, &fakeFileService{}, "toon"), "issue_1")

	// Homogeneous rows render as a toon table: one {keys} header, one line
	// per row.
	require.Contains(t, out, "{filename,source,url}")
	require.Contains(t, out, "login-form.png")
	require.Contains(t, out, "description")
}

func TestListEmpty(t *testing.T) {
	issue := &fakeIssueService{issue: &service.Issue{ID: "issue_1", Description: "no images"}}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, issue, &fakeCommentService{}, &fakeFileService{}, "json"), "issue_1")

	// An issue with no images is an empty array, not an error and not null.
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "zero images render as an empty array: %v", got)
	require.Empty(t, arr)
}

func TestListError(t *testing.T) {
	issue := &fakeIssueService{err: errors.New(`issue "BLA-999" not found`)}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, issue, &fakeCommentService{}, &fakeFileService{}, "json"), "BLA-999")

	require.ErrorContains(t, err, `issue "BLA-999" not found`)
}

func TestListCommentsError(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	comment := &fakeCommentService{err: errors.New("linear API returned 502: boom")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, issue, comment, &fakeFileService{}, "json"), "issue_1", "--comments")

	require.ErrorContains(t, err, "linear API returned 502")
}
