package tabs

import (
	"context"
	"os"
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

// fakeTabService is the hermetic service.SheetTabService double: it records
// every call and fails with the seeded error, if any. The embedded nil
// service.SheetTabService satisfies the interface if it grows; tabs leaves
// only ever call the three methods below.
type fakeTabService struct {
	service.SheetTabService

	addErr, deleteErr, renameErr error

	addID int64 // sheet id AddSheetTab returns

	addCalls, deleteCalls, renameCalls int

	addSpreadsheet, addTitle     string
	deleteSpreadsheet, deleteTab string
	renameSpreadsheet, renameOld string
	renameNew                    string
}

func (f *fakeTabService) AddSheetTab(_ context.Context, spreadsheetID, title string) (int64, error) {
	f.addCalls++
	f.addSpreadsheet, f.addTitle = spreadsheetID, title
	if f.addErr != nil {
		return 0, f.addErr
	}
	return f.addID, nil
}

func (f *fakeTabService) DeleteSheetTab(_ context.Context, spreadsheetID, title string) error {
	f.deleteCalls++
	f.deleteSpreadsheet, f.deleteTab = spreadsheetID, title
	return f.deleteErr
}

func (f *fakeTabService) RenameSheetTab(_ context.Context, spreadsheetID, oldTitle, newTitle string) error {
	f.renameCalls++
	f.renameSpreadsheet, f.renameOld, f.renameNew = spreadsheetID, oldTitle, newTitle
	return f.renameErr
}

// newLeafCmd builds a leaf against a fake service, ready to execute. The
// tabs leaves are writes: they carry no --format of their own, so an empty
// format resolves to JSON under the test harness (no TTY, no agent).
func newLeafCmd(build func(*app.Config, service.Dialer[service.SheetTabService]) *cobra.Command, svc *fakeTabService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// fakeNewSvc returns a service.Dialer[service.SheetTabService] handing out
// svc, so leaves run hermetically with no network and no real account store.
func fakeNewSvc(svc *fakeTabService) service.Dialer[service.SheetTabService] {
	return func(context.Context) (service.SheetTabService, error) { return svc, nil }
}

// seedSpreadsheetID is the spreadsheet id the tabs tests use everywhere.
const seedSpreadsheetID = "sheet_1"
