package image

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// pngFileService returns a fake FileService streaming content as
// image/png.
func pngFileService() *fakeFileService {
	return &fakeFileService{content: []byte("pngbytes"), contentType: "image/png"}
}

func TestDownloadJSON(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	file := pngFileService()
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, file, "json", fs), "issue_1", "--out-dir", "shots")

	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "two downloads render as an array: %v", got)
	require.Len(t, arr, 2)

	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "login-form.png", first["filename"], "an alt text with an extension keeps it")
	require.Equal(t, "shots/login-form.png", first["path"])
	require.Equal(t, float64(len("pngbytes")), first["bytes"])
	require.NotContains(t, first, "error")
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, first))

	second, ok := arr[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "graph.png", second["filename"], "a missing extension is fixed from the response's Content-Type")
	require.Equal(t, "shots/graph.png", second["path"])
	require.Equal(t, float64(len("pngbytes")), second["bytes"])

	// The bytes landed under the final names; the extensionless scratch was
	// renamed away.
	data, err := afero.ReadFile(fs, "shots/login-form.png")
	require.NoError(t, err)
	require.Equal(t, "pngbytes", string(data))
	require.True(t, exists(t, fs, "shots/graph.png"))
	require.False(t, exists(t, fs, "shots/graph"))
}

func TestDownloadDefaultOutDir(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "json", fs), "issue_1")

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	first := arr[0].(map[string]any)
	require.Equal(t, "login-form.png", first["path"], "the default --out-dir is the current directory, without a ./ prefix")
	require.True(t, exists(t, fs, "login-form.png"))
}

func TestDownloadNestedOutDir(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	fs := afero.NewMemMapFs()
	cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "json", fs), "issue_1", "--out-dir", "shots/2026/oct")

	require.True(t, exists(t, fs, "shots/2026/oct/login-form.png"), "the out dir is created as needed")
}

func TestDownloadTable(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "table", fs), "issue_1", "--out-dir", "shots")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "FILENAME")
	require.Contains(t, out, "BYTES")
	require.Contains(t, out, "PATH")
	require.Contains(t, out, "ERROR")
	require.Contains(t, out, "login-form.png")
	require.Contains(t, out, "graph.png")
	require.Contains(t, out, "shots/login-form.png")
}

func TestDownloadToon(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "toon", fs), "issue_1", "--out-dir", "shots")

	// Homogeneous rows render as a toon table: one {keys} header, one line
	// per row. The fixed name graph.png carries the extension from the
	// response.
	require.Contains(t, out, "{bytes,filename,path}")
	require.Contains(t, out, "login-form.png")
	require.Contains(t, out, "graph.png,shots/graph.png")
}

func TestDownloadEmpty(t *testing.T) {
	issue := &fakeIssueService{issue: &service.Issue{ID: "issue_1", Description: "no images"}}
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "json", fs), "issue_1")

	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "zero downloads render as an empty array: %v", got)
	require.Empty(t, arr)
}

func TestDownloadCollision(t *testing.T) {
	issue := seedIssue()
	issue.Description = "![graph](u-a) then ![graph](u-b)"
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, &fakeIssueService{issue: issue}, &fakeCommentService{}, pngFileService(), "json", fs), "issue_1")

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	require.Equal(t, "graph.png", arr[0].(map[string]any)["filename"])
	require.Equal(t, "graph-2.png", arr[1].(map[string]any)["filename"], "a repeated name gets a -2 suffix")
	require.True(t, exists(t, fs, "graph.png"))
	require.True(t, exists(t, fs, "graph-2.png"), "both images land, neither overwrites the other")
}

// TestDownloadNonAllowlistedHostRowFails: a URL the FileService seam
// refuses (only https://uploads.linear.app is fetched) fails exactly its
// row with the seam's named error — never silently — while the other
// downloads proceed and the command exits non-zero.
func TestDownloadNonAllowlistedHostRowFails(t *testing.T) {
	evil := "https://files.example.com/x.png"
	issue := seedIssue()
	issue.Description = "![shot](u-good.png) and ![leak](https://files.example.com/x.png)"
	file := pngFileService()
	file.errs = map[string]error{evil: errors.New("linear file download refused URL: only https://uploads.linear.app is allowed")}
	fs := afero.NewMemMapFs()
	out, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newDownloadCmd, &fakeIssueService{issue: issue}, &fakeCommentService{}, file, "json", fs), "issue_1", "--out-dir", "shots")

	require.ErrorContains(t, err, "image download: 1 of 2 downloads failed")

	arr, ok := cmdtest.DecodeJSON(t, rowsJSON(t, out)).([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	good := arr[0].(map[string]any)
	require.Equal(t, "shot.png", good["filename"])
	require.Contains(t, good, "path")
	require.NotContains(t, good, "error")
	failed := arr[1].(map[string]any)
	require.Equal(t, "leak", failed["filename"], "no response arrived, so no extension could be fixed")
	require.Equal(t, "linear file download refused URL: only https://uploads.linear.app is allowed", failed["error"])
	require.NotContains(t, failed, "path")
	require.NotContains(t, failed, "bytes")

	// The leaf passes every URL to the seam — the guard is the seam's — and a
	// failed row leaves no file behind.
	require.Equal(t, []string{"u-good.png", evil}, file.gotURLs)
	require.True(t, exists(t, fs, "shots/shot.png"))
	require.False(t, exists(t, fs, "shots/leak"))
}

// TestDownloadFailedFetchLeavesNoFile: a failed fetch fails its row with
// the seam's error and removes the scratch file, so no partial image
// remains.
func TestDownloadFailedFetchLeavesNoFile(t *testing.T) {
	issue := seedIssue()
	issue.Description = "![graph](u-1)"
	file := &fakeFileService{err: errors.New("linear API returned 502: boom")}
	fs := afero.NewMemMapFs()
	out, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newDownloadCmd, &fakeIssueService{issue: issue}, &fakeCommentService{}, file, "json", fs), "issue_1", "--out-dir", "shots")

	require.ErrorContains(t, err, "image download: 1 of 1 downloads failed")
	m, ok := cmdtest.DecodeJSON(t, rowsJSON(t, out)).(map[string]any)
	require.True(t, ok, "one failed download renders as a single object: %v", out)
	require.Equal(t, "graph", m["filename"], "no response arrived, so no extension could be fixed")
	require.Equal(t, "linear API returned 502: boom", m["error"])
	require.False(t, exists(t, fs, "shots/graph"))
}

// TestDownloadExtensionFixing: a missing extension is fixed from the
// response's Content-Type (parameters and case tolerated); an unknown
// type leaves the alt-text name unchanged.
func TestDownloadExtensionFixing(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        string
	}{
		{"png", "image/png", "graph.png"},
		{"jpeg maps to jpg", "image/jpeg", "graph.jpg"},
		{"webp", "image/webp", "graph.webp"},
		{"svg", "image/svg+xml", "graph.svg"},
		{"parameters tolerated", "image/png; charset=binary", "graph.png"},
		{"unknown type", "application/octet-stream", "graph"},
		{"no content type", "", "graph"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := seedIssue()
			issue.Description = "![graph](u-1)"
			file := &fakeFileService{content: []byte("bytes"), contentType: tt.contentType}
			fs := afero.NewMemMapFs()
			out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, &fakeIssueService{issue: issue}, &fakeCommentService{}, file, "json", fs), "issue_1")

			// One download renders as a single JSON object.
			m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
			require.True(t, ok)
			require.Equal(t, tt.want, m["filename"])
			require.True(t, exists(t, fs, tt.want))
		})
	}
}

// TestDownloadExistingExtensionKept: an alt text that already carries an
// extension keeps it — the response's type never renames an named file.
func TestDownloadExistingExtensionKept(t *testing.T) {
	issue := seedIssue()
	issue.Description = "![photo.jpg](u-1)"
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, &fakeIssueService{issue: issue}, &fakeCommentService{}, pngFileService(), "json", fs), "issue_1")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one download renders as a single object: %v", out)
	require.Equal(t, "photo.jpg", m["filename"])
	require.True(t, exists(t, fs, "photo.jpg"))
}

// TestDownloadEmptyAltFallsBack: an alt text that sanitizes to nothing
// falls back to a generic name, still extension-fixed and
// collision-suffixed.
func TestDownloadEmptyAltFallsBack(t *testing.T) {
	issue := seedIssue()
	issue.Description = "![](u-1) and ![](u-2)"
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, &fakeIssueService{issue: issue}, &fakeCommentService{}, pngFileService(), "json", fs), "issue_1")

	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Equal(t, "image.png", arr[0].(map[string]any)["filename"])
	require.Equal(t, "image-2.png", arr[1].(map[string]any)["filename"])
	require.True(t, exists(t, fs, "image.png"))
	require.True(t, exists(t, fs, "image-2.png"))
}

func TestDownloadDoesNotScanCommentsByDefault(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	comment := &fakeCommentService{comments: seedComments()}
	fs := afero.NewMemMapFs()
	cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, comment, pngFileService(), "json", fs), "issue_1")

	require.False(t, comment.dialed, "without --comments the comment service is never dialed")
}

func TestDownloadComments(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	comment := &fakeCommentService{comments: seedComments()}
	fs := afero.NewMemMapFs()
	out := cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, comment, pngFileService(), "json", fs), "issue_1", "--comments")

	// Two description images plus the fresh comment image (the repeated URL
	// deduplicated away).
	arr, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, arr, 3)
	require.Equal(t, "fresh.png", arr[2].(map[string]any)["filename"])
	require.True(t, exists(t, fs, "fresh.png"))
}

func TestDownloadIssueIDPassthrough(t *testing.T) {
	issue := &fakeIssueService{issue: seedIssue()}
	fs := afero.NewMemMapFs()
	cmdtest.RunCmd(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "json", fs), "BLA-123")

	require.Equal(t, "BLA-123", issue.gotID, "human identifiers pass through to the service")
}

func TestDownloadError(t *testing.T) {
	issue := &fakeIssueService{err: errors.New(`issue "BLA-999" not found`)}
	fs := afero.NewMemMapFs()
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newDownloadCmd, issue, &fakeCommentService{}, pngFileService(), "json", fs), "BLA-999")

	require.ErrorContains(t, err, `issue "BLA-999" not found`)
}
