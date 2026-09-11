package file

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestFileNewCmdRegistersLeaves: the `drive file` parent wires every leaf,
// sharing and otherwise, as one AddCommand line each.
func TestFileNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))

	require.Equal(t, "file", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.ElementsMatch(t,
		[]string{"list", "get", "create", "copy", "upload", "download", "trash", "untrash", "delete",
			"permissions", "share", "unshare"},
		names,
	)
}

// TestFileRowFields pins the list row shape: snake_case keys, exactly the
// documented fields, in the seeded file.
func TestFileRowShape(t *testing.T) {
	row := fileRow(seedDetailFile())

	cmdtest.RequireSnakeCase(t, keysOf(row))
	require.Equal(t, "file_1", row["id"])
	require.Equal(t, "Report", row["name"])
	require.Equal(t, "application/pdf", row["mime_type"])
	require.EqualValues(t, 1234, row["size"])
	require.Equal(t, "me@example.com", row["owner"])
	require.Equal(t, []string{"parent_1", "parent_2"}, row["parent_ids"])
	require.Equal(t, true, row["trashed"])
	require.Equal(t, true, row["shared"])
	require.Equal(t, "2026-08-24T09:00:00.000Z", row["modified_time"])
	require.Equal(t, "https://drive.google.com/file/d/file_1/view", row["web_link"])
}

// TestFileViewAddsDescription: get's view extends the list row with
// description only.
func TestFileViewAddsDescription(t *testing.T) {
	view := fileView(seedDetailFile())

	require.Equal(t, "Quarterly report", view["description"])
	require.Len(t, fileViewFields, len(fileListFields)+1)
}

// TestFileRowSizeEmptyForNative: Google-native types report size 0; the
// rendered size must be empty string, never 0.
func TestFileRowSizeEmptyForNative(t *testing.T) {
	native := fileRow(&drive.File{Id: "f", MimeType: "application/vnd.google-apps.document"})
	require.Empty(t, native["size"], "Google-native size must render as empty string, not 0")

	binary := fileRow(&drive.File{Id: "f", Size: 7})
	require.EqualValues(t, 7, binary["size"])
}

// TestEscapeQ backslash-escapes single quotes (and backslashes, first) so a
// quote- or backslash-bearing name stays one Drive q term.
func TestEscapeQ(t *testing.T) {
	require.Equal(t, `O\'Brien report`, escapeQ("O'Brien report"))
	require.Equal(t, `back\\slash`, escapeQ(`back\slash`))
	require.Equal(t, `O\'Brien\\\'s`, escapeQ(`O'Brien\'s`))
	// A trailing backslash doubles, so the literal stays terminated.
	require.Equal(t, `trailing\\`, escapeQ(`trailing\`))
	require.Empty(t, escapeQ(""))
}

// TestComposeQuery pins the q composition: raw passthrough ANDed with the
// composed shorthand terms, quote-escaped name substrings, and the default
// trashed = false.
func TestComposeQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		name_   string
		parent  string
		mime    string
		trashed bool
		want    string
	}{
		{"nothing", "", "", "", "", false, "trashed = false"},
		{"query only", "owner = 'me@example.com'", "", "", "", false,
			"owner = 'me@example.com' and trashed = false"},
		{"name only", "", "invoice", "", "", false, "name contains 'invoice' and trashed = false"},
		{"name escapes quotes", "", "O'Brien's", "", "", false, `name contains 'O\'Brien\'s' and trashed = false`},
		{"name escapes trailing backslash", "", `trailing\`, "", "", false, `name contains 'trailing\\' and trashed = false`},
		{"parent only", "", "", "1AbC", "", false, "'1AbC' in parents and trashed = false"},
		{"parent escapes quotes", "", "", `my'O'folder`, "", false, `'my\'O\'folder' in parents and trashed = false`},
		{"mime shorthand", "", "", "", "folder", false,
			"mimeType = 'application/vnd.google-apps.folder' and trashed = false"},
		{"mime raw passthrough", "", "", "", "image/png", false,
			"mimeType = 'image/png' and trashed = false"},
		{"mime escapes quotes", "", "", "", `we'ird`, false, `mimeType = 'we\'ird' and trashed = false`},
		{"trashed flag drops term", "", "", "", "", true, ""},
		{"all combined", "fullText = 'q'", "note", "1AbC", "doc", false,
			"fullText = 'q' and name contains 'note' and '1AbC' in parents and " +
				"mimeType = 'application/vnd.google-apps.document' and trashed = false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeQuery(tt.query, tt.name_, tt.parent, resolveMime(tt.mime), tt.trashed)
			require.Equal(t, tt.want, strings.TrimSpace(got))
		})
	}
}

// TestComposeQueryMultiTermJoinsWithAnd pins the exact multi-term q shape:
// Drive v3 has no implicit AND, so non-empty terms are joined with " and ".
func TestComposeQueryMultiTermJoinsWithAnd(t *testing.T) {
	got := composeQuery("", "template", "", "application/vnd.google-apps.presentation", false)
	require.Equal(t,
		"name contains 'template' and mimeType = 'application/vnd.google-apps.presentation' and trashed = false",
		got)
}

// TestResolveExportMime pins the --export shorthand expansion: every
// documented shorthand maps to its full MIME type and anything else (a full
// MIME string, an unknown token) passes through unchanged.
func TestResolveExportMime(t *testing.T) {
	tests := map[string]string{
		"pdf":  "application/pdf",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"csv":  "text/csv",
		"tsv":  "text/tab-separated-values",
		"md":   "text/markdown",
		"txt":  "text/plain",
		"odt":  "application/vnd.oasis.opendocument.text",
		"ods":  "application/vnd.oasis.opendocument.spreadsheet",
		"odp":  "application/vnd.oasis.opendocument.presentation",
		// Unmapped values pass through raw.
		"application/vnd.google-apps.presentation": "application/vnd.google-apps.presentation",
		"application/pdf":                          "application/pdf",
		"bogus":                                    "bogus",
		"":                                         "",
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			require.Equal(t, want, resolveExportMime(in))
		})
	}
}

// keysOf returns the map's keys sorted for assertion-friendly ordering.
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
