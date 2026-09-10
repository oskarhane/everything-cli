package tabs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

// fakeDocService is the hermetic service.DocService double the tabs tests
// use: it serves the seeded tab tree from ListDocTabs, records every write,
// and fails with the seeded error, if any. The embedded nil service.DocService
// satisfies the interface if it grows; the tabs leaves only ever call the
// four methods below.
type fakeDocService struct {
	service.DocService

	tabs []service.DocTab // served by ListDocTabs

	listErr, addErr, deleteErr, renameErr error

	dials int // every dial, so tests can tell a flag error (none) from a resolution error

	listID string // last ListDocTabs doc id

	addDocID, addTitle string // last AddDocTab call
	addTabID           string // tab id AddDocTab returns

	deleteDocID, deleteTabID string // last DeleteDocTab call
	deleteCalls              int

	renameDocID, renameTabID, renameTitle string // last RenameDocTab call
	renameCalls                           int
}

func (f *fakeDocService) ListDocTabs(_ context.Context, docID string) ([]service.DocTab, error) {
	f.listID = docID
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tabs, nil
}

func (f *fakeDocService) AddDocTab(_ context.Context, docID, title string) (string, error) {
	f.addDocID, f.addTitle = docID, title
	if f.addErr != nil {
		return "", f.addErr
	}
	return f.addTabID, nil
}

func (f *fakeDocService) DeleteDocTab(_ context.Context, docID, tabID string) error {
	f.deleteCalls++
	f.deleteDocID, f.deleteTabID = docID, tabID
	return f.deleteErr
}

func (f *fakeDocService) RenameDocTab(_ context.Context, docID, tabID, title string) error {
	f.renameCalls++
	f.renameDocID, f.renameTabID, f.renameTitle = docID, tabID, title
	return f.renameErr
}

// ResolveDocTab resolves the --tab key against the seeded tab tree with the
// real service's rules — exact tab ID first, then exact title — so the
// delete/rename leaves' tests can assert the resolved ID reached the write.
func (f *fakeDocService) ResolveDocTab(_ context.Context, _, key string) (service.DocTab, error) {
	if f.listErr != nil {
		return service.DocTab{}, f.listErr
	}
	for _, tab := range f.tabs {
		if tab.TabID == key {
			return tab, nil
		}
	}
	var matches []service.DocTab
	for _, tab := range f.tabs {
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

// fakeNewSvc returns a service.Dialer[service.DocService] handing out svc,
// so leaves run hermetically with no network and no real account store. It
// counts every dial on the fake.
func fakeNewSvc(svc *fakeDocService) service.Dialer[service.DocService] {
	return func(context.Context) (service.DocService, error) {
		svc.dials++
		return svc, nil
	}
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.DocService]) *cobra.Command, svc *fakeDocService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedDocID is the document id the tabs tests use everywhere.
const seedDocID = "doc_1"

// seedTabs returns a small realistic tab tree flattened depth-first — two
// root tabs and a nested child under the first — mirroring what
// service.ListDocTabs hands back.
func seedTabs() []service.DocTab {
	return []service.DocTab{
		{TabID: "t.main", Title: "Main", Index: 0, NestingLevel: 0},
		{TabID: "t.child", Title: "Appendix", Index: 0, NestingLevel: 1, ParentTabID: "t.main"},
		{TabID: "t.second", Title: "Notes", Index: 1, NestingLevel: 0},
	}
}
