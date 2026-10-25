package image

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// TestImageNewCmdRegistersLeaves: the `linear issue image` parent wires
// every leaf as one AddCommand line each.
func TestImageNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"),
		testDialers(&fakeIssueService{}, &fakeCommentService{}, &fakeFileService{}))

	require.Equal(t, "image", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.ElementsMatch(t, []string{"list", "download"}, names)
}

// testDialers hands out the fake services so leaves run hermetically with
// no network and no real account store; the comment dialer records that
// it was dialed so tests can pin the --comments behavior.
func testDialers(issue *fakeIssueService, comment *fakeCommentService, file *fakeFileService) Dialers {
	return Dialers{
		Issue:   func(context.Context) (service.IssueService, error) { return issue, nil },
		Comment: func(context.Context) (service.CommentService, error) { comment.dialed = true; return comment, nil },
		File:    func(context.Context) (service.FileService, error) { return file, nil },
	}
}

// newLeafCmd builds a leaf against fake services, ready to execute.
func newLeafCmd(build func(*app.Config, Dialers) *cobra.Command, issue *fakeIssueService, comment *fakeCommentService, file *fakeFileService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), testDialers(issue, comment, file))
}

// newLeafCmdWithFs builds a leaf on the given in-memory FS, for download
// assertions that read written files back.
func newLeafCmdWithFs(build func(*app.Config, Dialers) *cobra.Command, issue *fakeIssueService, comment *fakeCommentService, file *fakeFileService, format string, fs afero.Fs) *cobra.Command {
	return build(&app.Config{Format: format, Fs: fs}, testDialers(issue, comment, file))
}

// exists asserts a path exists on fs.
func exists(t *testing.T, fs afero.Fs, path string) bool {
	t.Helper()
	ok, err := afero.Exists(fs, path)
	require.NoError(t, err)
	return ok
}

// rowsJSON returns the stdout JSON of a RunCmdErr capture: cobra appends
// its "Error:" line to the same buffer after the rows, which the JSON
// decoder must not see.
func rowsJSON(t *testing.T, out string) string {
	t.Helper()
	before, _, found := strings.Cut(out, "\nError:")
	require.True(t, found, "expected cobra's error line after the rows: %q", out)
	return before
}

// fakeIssueService is the hermetic service.IssueService double: it serves
// one seeded issue and records the id GetIssue received. The other
// methods are stubs the image tree never calls.
type fakeIssueService struct {
	err   error // when set, every call fails
	issue *service.Issue
	gotID string
}

func (f *fakeIssueService) ListIssues(context.Context, service.IssueFilter) ([]service.Issue, error) {
	return nil, f.err
}

func (f *fakeIssueService) GetIssue(_ context.Context, id string) (*service.Issue, error) {
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.issue, nil
}

func (f *fakeIssueService) CreateIssue(context.Context, service.CreateIssueInput) (*service.Issue, error) {
	return nil, f.err
}

func (f *fakeIssueService) UpdateIssue(context.Context, string, service.UpdateIssueInput) (*service.Issue, error) {
	return nil, f.err
}

// fakeCommentService is the hermetic service.CommentService double: it
// serves the seeded comment list and records the issue id it received.
type fakeCommentService struct {
	err      error // when set, every call fails
	comments []service.Comment
	dialed   bool
	gotID    string
}

func (f *fakeCommentService) ListComments(_ context.Context, issueID string) ([]service.Comment, error) {
	f.gotID = issueID
	if f.err != nil {
		return nil, f.err
	}
	return f.comments, nil
}

func (f *fakeCommentService) CreateComment(context.Context, string, service.CreateCommentInput) (*service.Comment, error) {
	return nil, f.err
}

// fakeFileService is the hermetic service.FileService double: it streams
// the seeded bytes with the seeded content type and records every URL it
// received; errs fails exactly its URLs (so one row can fail while
// others succeed), and err fails every download.
type fakeFileService struct {
	err         error            // when set, every download fails
	errs        map[string]error // per-URL failures, like the allowlist guard
	content     []byte
	contentType string
	gotURLs     []string
}

func (f *fakeFileService) DownloadFileTo(ctx context.Context, fileURL string, w io.Writer) error {
	_, err := f.DownloadFileMeta(ctx, fileURL, w)
	return err
}

func (f *fakeFileService) DownloadFileMeta(_ context.Context, fileURL string, w io.Writer) (service.FileMeta, error) {
	f.gotURLs = append(f.gotURLs, fileURL)
	if err, ok := f.errs[fileURL]; ok {
		return service.FileMeta{}, err
	}
	if f.err != nil {
		return service.FileMeta{}, f.err
	}
	n, err := w.Write(f.content)
	return service.FileMeta{ContentType: f.contentType, Bytes: int64(n)}, err
}

// seedIssue returns one issue whose description references two storage
// images: one alt text carrying its extension, one without it (fixed on
// download from the response's Content-Type).
func seedIssue() *service.Issue {
	return &service.Issue{
		ID:          "issue_1",
		Identifier:  "BLA-123",
		Title:       "Fix login redirect",
		Description: "Before:\n\n![login-form.png](https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000001)\n\nAfter:\n\n![graph](https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000002)",
	}
}

// seedComments returns comments for the dedup cases: one repeating the
// description's second URL under a different alt text, one adding a
// fresh URL, one with no images.
func seedComments() []service.Comment {
	return []service.Comment{
		{ID: "comment_1", Body: "repost: ![dupe](https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000002)"},
		{ID: "comment_2", Body: "and a fresh one: ![fresh.png](https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000003)"},
		{ID: "comment_3", Body: "no images here"},
	}
}
