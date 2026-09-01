package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestGetJSON(t *testing.T) {
	svc := &fakeService{files: []*drive.File{seedDetailFile()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "file_1")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, fileViewFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "file_1", row["id"])
	require.Equal(t, "Report", row["name"])
	require.Equal(t, "application/pdf", row["mime_type"])
	require.EqualValues(t, 1234, row["size"])
	require.Equal(t, "me@example.com", row["owner"])
	require.Equal(t, []any{"parent_1", "parent_2"}, row["parent_ids"])
	require.Equal(t, true, row["trashed"])
	require.Equal(t, true, row["shared"])
	require.Equal(t, "2026-08-24T09:00:00.000Z", row["modified_time"])
	require.Equal(t, "https://drive.google.com/file/d/file_1/view", row["web_link"])
	require.Equal(t, "Quarterly report", row["description"])
}

func TestGetTable(t *testing.T) {
	svc := &fakeService{files: []*drive.File{seedDetailFile()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "file_1")

	// go-pretty StyleLight upper-cases headers; parent_ids renders compactly.
	for _, header := range []string{
		"ID", "NAME", "MIME_TYPE", "SIZE", "OWNER", "PARENT_IDS", "TRASHED", "SHARED", "MODIFIED_TIME", "WEB_LINK", "DESCRIPTION",
	} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "me@example.com")
	require.Contains(t, out, "parent_1,parent_2")
}

func TestGetSizeEmptyForNative(t *testing.T) {
	file := seedDetailFile()
	file.MimeType = "application/vnd.google-apps.document"
	file.Size = 0
	svc := &fakeService{files: []*drive.File{file}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "file_1")

	row := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.Empty(t, row["size"], "Google-native size must render empty, not 0")
}

func TestGetMissingID(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "file_404")

	require.Contains(t, err.Error(), "file file_404 not found")
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "file_1")

	require.Contains(t, err.Error(), "googleapi: Error 500")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
