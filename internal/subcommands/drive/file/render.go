package file

import (
	"strings"

	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// fileListFields is the file list row field order for table output; the same
// names are the snake_case JSON and TOON keys. fileViewFields is the single
// file field order (get adds description). go-pretty's StyleLight upper-cases
// the headers when rendering.
var (
	fileListFields = []string{
		"id", "name", "mime_type", "size", "owner",
		"parent_ids", "trashed", "shared", "modified_time", "web_link",
	}
	fileViewFields = []string{
		"id", "name", "mime_type", "size", "owner",
		"parent_ids", "trashed", "shared", "modified_time", "web_link", "description",
	}
)

// mimeShorthands maps --mime/--type shorthand values to Drive MIME types.
// Any other value passes through raw, so a full MIME string works too.
var mimeShorthands = map[string]string{
	"folder": "application/vnd.google-apps.folder",
	"doc":    "application/vnd.google-apps.document",
	"sheet":  "application/vnd.google-apps.spreadsheet",
	"slide":  "application/vnd.google-apps.presentation",
}

// defaultExportMimes maps the Google-native types with a text default to the
// export MIME the download leaf uses when --export is not set. Sheet CSV/TSV
// exports cover the FIRST SHEET ONLY (see download.go help text).
var defaultExportMimes = map[string]string{
	"application/vnd.google-apps.document":     "text/plain",
	"application/vnd.google-apps.spreadsheet":  "text/csv",
	"application/vnd.google-apps.presentation": "text/plain",
}

// supportedExports lists the export MIME types files.export accepts per
// Google-native family (the export matrix from the Drive API docs).
// Google-native types have no downloadable binary, so a download request on
// one of them needs one of these.
const supportedExports = "docs: text/plain, text/markdown, application/pdf, application/rtf, " +
	"application/epub+zip, application/zip, application/vnd.openxmlformats-officedocument.wordprocessingml.document, " +
	"application/vnd.oasis.opendocument.text; sheets: text/csv (first sheet only), text/tab-separated-values (first sheet only), " +
	"application/pdf, application/zip, application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, " +
	"application/vnd.oasis.opendocument.spreadsheet; slides: text/plain, application/pdf, " +
	"application/vnd.openxmlformats-officedocument.presentationml.presentation, application/vnd.oasis.opendocument.presentation; " +
	"drawings: image/png, image/jpeg, image/svg+xml, application/pdf"

// fileRow maps one file to its output row. size renders "" for Google-native
// types (their Size is 0) instead of a misleading 0; parent_ids keeps the
// array for JSON/TOON and is compacted for table cells.
func fileRow(f *drive.File) map[string]any {
	owner := ""
	if len(f.Owners) > 0 && f.Owners[0] != nil {
		owner = f.Owners[0].EmailAddress
	}
	return map[string]any{
		"id":            f.Id,
		"name":          f.Name,
		"mime_type":     f.MimeType,
		"size":          sizeString(f.Size),
		"owner":         owner,
		"parent_ids":    f.Parents,
		"trashed":       f.Trashed,
		"shared":        f.Shared,
		"modified_time": f.ModifiedTime,
		"web_link":      f.WebViewLink,
	}
}

// fileView extends fileRow with the description, the field users read a
// single file for.
func fileView(f *drive.File) map[string]any {
	row := fileRow(f)
	row["description"] = f.Description
	return row
}

// sizeString renders Size, or "" when the file is Google-native (size 0).
func sizeString(n int64) any {
	if n == 0 {
		return ""
	}
	return n
}

// escapeQ escapes s for a quoted Drive q literal using backslash escaping:
// backslashes first, then single quotes: a name with apostrophes like
// O'Brien's is emitted as the q literal 'O\'Brien\'s', which stays one term
// ("Escape single quotes in queries with \'"). Drive q does NOT use the
// quote-doubling grammar — that belongs to A1-quoted sheet titles, which
// service.DoubleSingleQuotes covers.
func escapeQ(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `'`, `\'`)
}

// composeQuery builds the API's q parameter from the raw --query passthrough
// plus the composed shorthand terms, ANDed (space-joined in Drive q syntax).
// --trashed=false adds trashed = false so trashed files are excluded by
// default; --trashed leaves the term off so both are returned.
func composeQuery(query, name, parentID, mimeType string, trashed bool) string {
	var terms []string
	if query != "" {
		terms = append(terms, query)
	}
	if name != "" {
		terms = append(terms, "name contains '"+escapeQ(name)+"'")
	}
	if parentID != "" {
		terms = append(terms, "'"+escapeQ(parentID)+"' in parents")
	}
	if mimeType != "" {
		terms = append(terms, "mimeType = '"+escapeQ(mimeType)+"'")
	}
	if !trashed {
		terms = append(terms, "trashed = false")
	}
	return strings.Join(terms, " ")
}

// resolveMime expands a --mime shorthand to its Drive MIME type; other values
// pass through raw, so full MIME strings work.
func resolveMime(value string) string {
	if mime, ok := mimeShorthands[value]; ok {
		return mime
	}
	return value
}

// printFileList renders zero or more files: a JSON/TOON array, or a table
// with one row per file, in the resolved output format.
func printFileList(cmd *cobra.Command, cfg *app.Config, files []*drive.File) {
	rows := make([]map[string]any, 0, len(files))
	for _, f := range files {
		rows = append(rows, fileRow(f))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), fileListFields, rows, tableRows(rows))
}

// printFileView renders a single file: an object in JSON/TOON, a one-row
// table with the id list compacted into a cell.
func printFileView(cmd *cobra.Command, cfg *app.Config, f *drive.File) {
	view := fileView(f)
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), fileViewFields, view, []map[string]any{compactRow(view)})
}

// tableRows copies rows with parent_ids compacted for table cells; JSON and
// TOON keep the array.
func tableRows(rows []map[string]any) []map[string]any {
	compacted := make([]map[string]any, len(rows))
	for i, row := range rows {
		compacted[i] = compactRow(row)
	}
	return compacted
}

// compactRow copies row with parent_ids joined to a single string.
func compactRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		if k == "parent_ids" {
			if ids, ok := v.([]string); ok {
				v = strings.Join(ids, ",")
			}
		}
		out[k] = v
	}
	return out
}
