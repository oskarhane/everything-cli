package values

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeValuesService is the hermetic service.SheetValuesService double: it
// serves seeded values and records every write for assertions. The embedded
// nil service.SheetValuesService satisfies the label-shaped seam the parent
// hands down; values leaves never call those methods, so it stays nil.
type fakeValuesService struct {
	service.SheetValuesService

	getErr    error
	appendErr error
	updateErr error
	clearErr  error

	values map[string][][]any // served by GetValues, keyed by A1 range

	getID, getRange string

	appendID, appendRange, appendOption, appendUpdated string
	appended                                           [][]any
	appendRows, appendCols                             int64

	updateID, updateRange, updateOption, updateUpdated string
	updated                                            [][]any
	updatedCells                                       int64

	clearID, clearRange, clearReturned string
}

func (f *fakeValuesService) GetValues(_ context.Context, id, a1Range string) ([][]any, error) {
	f.getID, f.getRange = id, a1Range
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.values[a1Range], nil
}

func (f *fakeValuesService) AppendValues(_ context.Context, id, a1Range string, values [][]any, inputOption string) (string, int64, int64, error) {
	f.appendID, f.appendRange, f.appendOption = id, a1Range, inputOption
	f.appended = values
	if f.appendErr != nil {
		return "", 0, 0, f.appendErr
	}
	return f.appendUpdated, f.appendRows, f.appendCols, nil
}

func (f *fakeValuesService) UpdateValues(_ context.Context, id, a1Range string, values [][]any, inputOption string) (string, int64, error) {
	f.updateID, f.updateRange, f.updateOption = id, a1Range, inputOption
	f.updated = values
	if f.updateErr != nil {
		return "", 0, f.updateErr
	}
	return f.updateUpdated, f.updatedCells, nil
}

func (f *fakeValuesService) ClearValues(_ context.Context, id, a1Range string) (string, error) {
	f.clearID, f.clearRange = id, a1Range
	if f.clearErr != nil {
		return "", f.clearErr
	}
	return f.clearReturned, nil
}

// fakeNewSvc returns a service.Dialer[service.SheetValuesService] handing out
// svc, so leaves run hermetically with no network and no real account store.
func fakeNewSvc(svc *fakeValuesService) service.Dialer[service.SheetValuesService] {
	return func(context.Context) (service.SheetValuesService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.SheetValuesService]) *cobra.Command, svc *fakeValuesService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// newLeafCmdWithValuesFile builds a leaf against a fake service whose config
// FS is pre-seeded with a values file at path, for --values-file tests.
func newLeafCmdWithFs(build func(*app.Config, service.Dialer[service.SheetValuesService]) *cobra.Command, svc *fakeValuesService, format string, seed func(afero.Fs)) *cobra.Command {
	cfg := cmdtest.NewTestConfig(format)
	if seed != nil {
		seed(cfg.Fs)
	}
	return build(cfg, fakeNewSvc(svc))
}

// writeTestFile seeds one file on fs.
func writeTestFile(t *testing.T, fs afero.Fs, path, content string) {
	t.Helper()
	require.NoError(t, afero.WriteFile(fs, path, []byte(content), 0o600))
}

// seedSpreadsheetID is the spreadsheet id the values tests use everywhere.
const seedSpreadsheetID = "sheet_1"
