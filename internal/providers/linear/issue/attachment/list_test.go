package attachment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// seedAttachments returns one attachment carrying every rendered field
// (subtitle set) and one without a subtitle, pinning both JSON row shapes.
func seedAttachments() []service.Attachment {
	return []service.Attachment{
		{
			ID:        "attachment_1",
			Title:     "RFC",
			URL:       "https://example.com/rfc",
			Subtitle:  "Design doc",
			CreatedAt: "2026-08-01T10:00:00.000Z",
			UpdatedAt: "2026-08-02T10:00:00.000Z",
		},
		{
			ID:        "attachment_2",
			Title:     "Changelog",
			URL:       "https://example.com/changelog",
			CreatedAt: "2026-08-03T10:00:00.000Z",
			UpdatedAt: "2026-08-03T10:00:00.000Z",
		},
	}
}

func TestListJSON(t *testing.T) {
	svc := &fakeService{attachments: seedAttachments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "issue_1")

	require.Equal(t, "issue_1", svc.attachmentsIssueID)
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "two attachments render as an array: %v", got)
	require.Len(t, arr, 2)

	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "attachment_1", first["id"])
	require.Equal(t, "RFC", first["title"])
	require.Equal(t, "https://example.com/rfc", first["url"])
	require.Equal(t, "Design doc", first["subtitle"])
	require.Contains(t, first, "created_at")
	require.NotContains(t, first, "createdAt")

	second, ok := arr[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "attachment_2", second["id"])
	require.NotContains(t, second, "subtitle", "an empty subtitle is omitted")
}

func TestListSingleAttachmentRendersAsOneObject(t *testing.T) {
	svc := &fakeService{attachments: seedAttachments()[:1]}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "issue_1")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one attachment renders as a single object: %v", out)
	require.Equal(t, "attachment_1", m["id"])
}

func TestListIssueIDPassthrough(t *testing.T) {
	svc := &fakeService{attachments: seedAttachments()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "BLA-123")

	require.Equal(t, "BLA-123", svc.attachmentsIssueID, "human identifiers pass through to the service")
}

func TestListTable(t *testing.T) {
	svc := &fakeService{attachments: seedAttachments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"), "issue_1")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "CREATED_AT")
	require.Contains(t, out, "TITLE")
	require.Contains(t, out, "URL")
	require.Contains(t, out, "RFC")
	require.Contains(t, out, "Changelog")
	require.Contains(t, out, "https://example.com/rfc")
}

func TestListToon(t *testing.T) {
	svc := &fakeService{attachments: seedAttachments()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "toon"), "issue_1")

	require.Contains(t, out, "title: RFC")
	// toon quotes values containing characters like `:` and `.`.
	require.Contains(t, out, `created_at: "2026-08-01T10:00:00.000Z"`)
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "issue_1")

	// An issue with no attachments is an empty array, not an error and not
	// null.
	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "zero attachments render as an empty array: %v", got)
	require.Empty(t, arr)
}

func TestListError(t *testing.T) {
	svc := &fakeService{err: errors.New(`issue "BLA-999" not found`)}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"), "BLA-999")

	require.ErrorContains(t, err, `issue "BLA-999" not found`)
}
