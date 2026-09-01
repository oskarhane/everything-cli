package docs

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeDocService is the hermetic service.DocService double: it serves the
// seeded export text and records every write for assertions. The embedded
// nil service.DocService satisfies any surface the parent hands down that
// these leaves never call, so it stays nil.
type fakeDocService struct {
	service.DocService

	err          error  // when set, every call fails
	docText      string // served by GetDocText
	appendedID   string
	appendedText string
	insertID     string
	insertText   string
	insertIndex  int64
	replaceID    string
	replaceFind  string
	replaceWith  string
	replaceCase  bool
	replaceCount int
}

func (f *fakeDocService) GetDocText(_ context.Context, docID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.docText, nil
}

func (f *fakeDocService) AppendDocText(_ context.Context, docID, text string) error {
	if f.err != nil {
		return f.err
	}
	f.appendedID, f.appendedText = docID, text
	return nil
}

func (f *fakeDocService) InsertDocText(_ context.Context, docID, text string, index int64) error {
	if f.err != nil {
		return f.err
	}
	f.insertID, f.insertText, f.insertIndex = docID, text, index
	return nil
}

func (f *fakeDocService) ReplaceDocText(_ context.Context, docID, find, replaceWith string, matchCase bool) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.replaceID, f.replaceFind, f.replaceWith, f.replaceCase = docID, find, replaceWith, matchCase
	return f.replaceCount, nil
}

// fakeNewSvc returns a service.Dialer[service.DocService] handing out svc, so
// content leaves run hermetically with no network and no real account store.
func fakeNewSvc(svc *fakeDocService) service.Dialer[service.DocService] {
	return func(context.Context) (service.DocService, error) { return svc, nil }
}

// fakeNewFileSvc returns a service.Dialer[service.FileService] handing out
// svc, for the delete leaf, which rides the Drive surface instead of Docs.
func fakeNewFileSvc(svc *cmdtest.DeleteRecorder) service.Dialer[service.FileService] {
	return func(context.Context) (service.FileService, error) { return svc, nil }
}

// newLeafCmd builds a content leaf against a fake DocService, ready to
// execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.DocService]) *cobra.Command, svc *fakeDocService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// newLeafCmdWithFs builds a content leaf against a fake service and a
// supplied FS, for leaves that read or write files (get --out, --text-file).
func newLeafCmdWithFs(build func(*app.Config, service.Dialer[service.DocService]) *cobra.Command, svc *fakeDocService, format string, fs afero.Fs) *cobra.Command {
	cfg := cmdtest.NewTestConfig(format)
	cfg.Fs = fs
	return build(cfg, fakeNewSvc(svc))
}

// newFileLeafCmd builds the delete leaf against a fake FileService, ready to
// execute.
func newFileLeafCmd(build func(*app.Config, service.Dialer[service.FileService]) *cobra.Command, svc *cmdtest.DeleteRecorder, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewFileSvc(svc))
}

// seedDocText returns a small realistic document export, including control
// bytes, for the get streaming tests (bytes must pass through verbatim).
func seedDocText() string {
	return "Meeting notes\t2026\n\x1fSection two\n"
}

// seedTextFile writes text to a file on the test FS and returns its path.
func seedTextFile(t *testing.T, fs afero.Fs, path, content string) {
	t.Helper()
	require.NoError(t, afero.WriteFile(fs, path, []byte(content), 0o644))
}

// readAll returns the full contents of a file on the test FS.
func readAll(t *testing.T, fs afero.Fs, path string) []byte {
	t.Helper()
	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	return data
}

// errAPI stands in for a Google API failure any leaf must propagate.
var errAPI = errors.New("googleapi: Error 403: access denied")
