package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, fileListFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "file_1", first["id"])
	require.Equal(t, "application/pdf", first["mime_type"])
	// Blob files carry their size; Google-native types render it empty.
	require.EqualValues(t, 42, first["size"])
	require.Equal(t, []any{"folder_1"}, first["parent_ids"])
}

// TestListRendersTrashed pins the trashed column end to end: the seeded
// trashed file must render trashed=true in JSON (the service wire tests guard
// that the API projection actually returns the field), while an untrashed file
// stays false.
func TestListRendersTrashed(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)

	trashed, ok := rows[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "file_3", trashed["id"])
	require.Equal(t, true, trashed["trashed"])

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, first["trashed"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"))

	for _, header := range []string{
		"ID", "NAME", "MIME_TYPE", "SIZE", "OWNER", "PARENT_IDS", "TRASHED", "SHARED", "MODIFIED_TIME", "WEB_LINK",
	} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "report.pdf")
	require.Contains(t, out, "folder_1")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
}

func TestListComposesQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"query only", []string{"--query", "fullText = 'invoice'"}, "fullText = 'invoice' and trashed = false"},
		{"query shorthand", []string{"-q", "owner = 'me'"}, "owner = 'me' and trashed = false"},
		{"name only", []string{"--name", "Q3 report"}, "name contains 'Q3 report' and trashed = false"},
		{"name with quotes", []string{"--name", "O'Brien's"}, `name contains 'O\'Brien\'s' and trashed = false`},
		{"parent only", []string{"--parent", "1AbC"}, "'1AbC' in parents and trashed = false"},
		{"parent with quotes", []string{"--parent", `my'O'folder`}, `'my\'O\'folder' in parents and trashed = false`},
		{"mime with quotes", []string{"--mime", `we'ird`}, `mimeType = 'we\'ird' and trashed = false`},
		{"mime folder", []string{"--mime", "folder"}, "mimeType = 'application/vnd.google-apps.folder' and trashed = false"},
		{"mime doc", []string{"--mime", "doc"}, "mimeType = 'application/vnd.google-apps.document' and trashed = false"},
		{"mime sheet", []string{"--mime", "sheet"}, "mimeType = 'application/vnd.google-apps.spreadsheet' and trashed = false"},
		{"mime slide", []string{"--mime", "slide"}, "mimeType = 'application/vnd.google-apps.presentation' and trashed = false"},
		{"mime raw", []string{"--mime", "image/png"}, "mimeType = 'image/png' and trashed = false"},
		{"trashed flag", []string{"--trashed"}, ""},
		{"all combined", []string{"-q", "owner = 'me'", "--name", "note", "--parent", "1AbC", "--mime", "sheet"},
			"owner = 'me' and name contains 'note' and '1AbC' in parents and " +
				"mimeType = 'application/vnd.google-apps.spreadsheet' and trashed = false"},
		{"no filters", nil, "trashed = false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{files: seedFiles()}
			cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), tt.args...)
			require.Equal(t, tt.want, svc.listQ)
		})
	}
}

func TestListHonorsMax(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--max", "1")

	require.EqualValues(t, 1, svc.listMax)
	// A single row renders as one JSON object, not a one-element array.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	require.Equal(t, "file_1", row["id"])
}

func TestListMaxZeroIsUncapped(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--max", "0")

	require.EqualValues(t, 0, svc.listMax)
	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, rows, 3)
}

func TestListDefaultMax(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.EqualValues(t, 25, svc.listMax)
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403: access denied")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestListRequiresNoArgs(t *testing.T) {
	svc := &fakeService{files: seedFiles()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"), "unexpected")

	require.Contains(t, err.Error(), "unknown command")
}
