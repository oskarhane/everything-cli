package docs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
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

	err           error            // when set, every call fails
	docText       string           // served by GetDocText
	docTabs       []service.DocTab // served by ListDocTabs
	docTabText    string           // served by GetDocTabText
	tabReadID     string           // tab key the last GetDocTabText received
	appendedID    string
	appendedText  string
	appendedTabID string
	insertID      string
	insertText    string
	insertIndex   int64
	insertTabID   string
	replaceID     string
	replaceFind   string
	replaceWith   string
	replaceCase   bool
	replaceCount  int
}

func (f *fakeDocService) GetDocText(_ context.Context, docID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.docText, nil
}

func (f *fakeDocService) AppendDocText(_ context.Context, docID, text, tabKey string) error {
	if f.err != nil {
		return f.err
	}
	if err := f.resolveTabKey(tabKey); err != nil {
		return err
	}
	f.appendedID, f.appendedText, f.appendedTabID = docID, text, tabKey
	return nil
}

func (f *fakeDocService) InsertDocText(_ context.Context, docID, text string, index int64, tabID string) error {
	if f.err != nil {
		return f.err
	}
	f.insertID, f.insertText, f.insertIndex, f.insertTabID = docID, text, index, tabID
	return nil
}

func (f *fakeDocService) ReplaceDocText(_ context.Context, docID, find, replaceWith string, matchCase bool) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.replaceID, f.replaceFind, f.replaceWith, f.replaceCase = docID, find, replaceWith, matchCase
	return f.replaceCount, nil
}

// ListDocTabs serves the seeded tab list; the leaves read it to resolve a
// --tab key (insert) or not at all (get/append forward the key to the
// service, which resolves it).
func (f *fakeDocService) ListDocTabs(context.Context, string) ([]service.DocTab, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.docTabs, nil
}

// GetDocTabText serves the seeded tab render and records the tab key the
// leaf forwarded.
func (f *fakeDocService) GetDocTabText(_ context.Context, _, tabKey string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if err := f.resolveTabKey(tabKey); err != nil {
		return "", err
	}
	f.tabReadID = tabKey
	return f.docTabText, nil
}

// resolveTabKey applies the real service's tab-key contract to the seeded
// tabs, so unknown-key tests exercise the leaves' error propagation the way
// a real dial would. With no tabs seeded the check is inert, keeping the
// older tests' dumb-fake behavior.
func (f *fakeDocService) resolveTabKey(key string) error {
	if len(f.docTabs) == 0 {
		return nil
	}
	_, err := f.ResolveDocTab(context.Background(), "", key)
	return err
}

// ResolveDocTab serves the leaf-side resolution insert needs: it applies the
// seeded tab contract and returns the matched tab (inert with no tabs
// seeded, keeping the older tests' dumb-fake behavior).
func (f *fakeDocService) ResolveDocTab(_ context.Context, _, key string) (service.DocTab, error) {
	if len(f.docTabs) == 0 {
		return service.DocTab{}, nil
	}
	for _, tab := range f.docTabs {
		if tab.TabID == key {
			return tab, nil
		}
	}
	var matches []service.DocTab
	for _, tab := range f.docTabs {
		if tab.Title == key {
			matches = append(matches, tab)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return service.DocTab{}, fmt.Errorf("no tab with ID or title %q", key)
	default:
		ids := make([]string, 0, len(matches))
		for _, tab := range matches {
			ids = append(ids, tab.TabID)
		}
		return service.DocTab{}, fmt.Errorf("tab title %q is ambiguous: matches tabs %s; use a tab ID",
			key, strings.Join(ids, ", "))
	}
}

// The three tab-mutation stubs below are untouched by any docs leaf yet, but
// the interface grew, so the fake must carry them (the embedded nil
// DocService leaves them missing otherwise).
func (f *fakeDocService) AddDocTab(context.Context, string, string) (string, error) {
	return "", nil
}

func (f *fakeDocService) DeleteDocTab(context.Context, string, string) error {
	return nil
}

func (f *fakeDocService) RenameDocTab(context.Context, string, string, string) error {
	return nil
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

// seedDocTabs returns the tab list the --tab tests resolve keys against: a
// root tab plus its child tab, so the listing mirrors a realistic
// depth-first flattening.
func seedDocTabs() []service.DocTab {
	return []service.DocTab{
		{TabID: "t.abc123", Title: "Meeting notes"},
		{TabID: "t.def456", Title: "Changelog", Index: 1, NestingLevel: 1, ParentTabID: "t.abc123"},
	}
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
