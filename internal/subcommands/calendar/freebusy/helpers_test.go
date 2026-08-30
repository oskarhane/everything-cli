package freebusy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// frozenNow anchors every relative window and default in the tests.
var frozenNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// freezeNow pins the package clock for the test and restores it after.
func freezeNow(t *testing.T) {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time { return frozenNow }
	t.Cleanup(func() { nowFunc = original })
}

// fakeFreeBusyService is the hermetic service.FreeBusyService double: it
// serves a seeded calendar list and freebusy response, and records the query
// params and list call for assertions.
type fakeFreeBusyService struct {
	entries    []*calendar.CalendarListEntry // served by ListCalendarList
	resp       *calendar.FreeBusyResponse    // served by QueryFreeBusy
	listCalled bool
	listErr    error
	params     []service.QueryFreeBusyParams
	queryErr   error
}

func (f *fakeFreeBusyService) ListCalendarList(context.Context) ([]*calendar.CalendarListEntry, error) {
	f.listCalled = true
	return f.entries, f.listErr
}

func (f *fakeFreeBusyService) QueryFreeBusy(_ context.Context, params service.QueryFreeBusyParams) (*calendar.FreeBusyResponse, error) {
	f.params = append(f.params, params)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.resp == nil {
		return &calendar.FreeBusyResponse{}, nil
	}
	return f.resp, nil
}

// fakeNewSvc returns a service.Dialer[service.FreeBusyService] handing out svc, so the leaf runs
// hermetically with no network and no real account store.
func fakeNewSvc(svc *fakeFreeBusyService) service.Dialer[service.FreeBusyService] {
	return func(context.Context) (service.FreeBusyService, error) { return svc, nil }
}

// newCmd builds the freebusy leaf against a fake service, ready to execute.
func newCmd(svc *fakeFreeBusyService, format string) *cobra.Command {
	return NewCmd(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}
