package calendarlist

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.CalendarService double: it serves
// seeded calendars and records writes for assertions. The acl methods are
// stubs this subtree never exercises.
type fakeService struct {
	entries     []*calendar.CalendarListEntry // served by ListCalendarList and GetCalendarList
	listErr     error                         // when set, ListCalendarList fails
	getEntryErr error                         // when set, GetCalendarList always fails

	getCal       *calendar.Calendar // served by GetCalendar
	getCalErr    error
	inserted     *calendar.Calendar // last InsertCalendar request
	insertErr    error
	patchedCalID string             // last PatchCalendar id
	patchedCal   *calendar.Calendar // last PatchCalendar request
	patchCalErr  error
	patchEntryID string                      // last PatchCalendarList id
	patchEntry   *calendar.CalendarListEntry // last PatchCalendarList request
	deleteCalled bool
	deletedID    string
	deleteErr    error
}

func (f *fakeService) ListCalendarList(context.Context) ([]*calendar.CalendarListEntry, error) {
	return f.entries, f.listErr
}

func (f *fakeService) GetCalendarList(_ context.Context, id string) (*calendar.CalendarListEntry, error) {
	if f.getEntryErr != nil {
		return nil, f.getEntryErr
	}
	for _, e := range f.entries {
		if e.Id == id {
			return e, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: calendar list entry %s not found", id)
}

func (f *fakeService) PatchCalendarList(_ context.Context, id string, entry *calendar.CalendarListEntry) (*calendar.CalendarListEntry, error) {
	f.patchEntryID = id
	f.patchEntry = entry
	return &calendar.CalendarListEntry{Id: id, ColorId: entry.ColorId}, nil
}

func (f *fakeService) GetCalendar(_ context.Context, _ string) (*calendar.Calendar, error) {
	if f.getCalErr != nil {
		return nil, f.getCalErr
	}
	return f.getCal, nil
}

func (f *fakeService) InsertCalendar(_ context.Context, cal *calendar.Calendar) (*calendar.Calendar, error) {
	if f.insertErr != nil {
		return nil, f.insertErr
	}
	f.inserted = cal
	return &calendar.Calendar{Id: "cal_99", Summary: cal.Summary, Description: cal.Description, TimeZone: cal.TimeZone}, nil
}

func (f *fakeService) PatchCalendar(_ context.Context, id string, cal *calendar.Calendar) (*calendar.Calendar, error) {
	if f.patchCalErr != nil {
		return nil, f.patchCalErr
	}
	f.patchedCalID = id
	f.patchedCal = cal
	return cal, nil
}

func (f *fakeService) DeleteCalendar(_ context.Context, id string) error {
	f.deleteCalled = true
	f.deletedID = id
	return f.deleteErr
}

func (f *fakeService) ListAcl(context.Context, string) ([]*calendar.AclRule, error) {
	return nil, nil
}

func (f *fakeService) InsertAcl(_ context.Context, _ string, rule *calendar.AclRule) (*calendar.AclRule, error) {
	return rule, nil
}

func (f *fakeService) DeleteAcl(context.Context, string, string) error {
	return nil
}

// fakeNewSvc returns a service.Dialer[service.CalendarService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.CalendarService] {
	return func(context.Context) (service.CalendarService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.CalendarService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedEntries returns a small realistic calendar list.
func seedEntries() []*calendar.CalendarListEntry {
	return []*calendar.CalendarListEntry{
		{Id: "oskar@example.com", Summary: "oskar@example.com", TimeZone: "Europe/Stockholm", Primary: true, ColorId: "tomato", Description: "Work calendar"},
		{Id: "abc123.group.calendar.google.com", Summary: "Team PTO", TimeZone: "Europe/Stockholm", ColorId: "banana"},
	}
}
