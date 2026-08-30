package calendarlist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newTestConfig returns a config forcing the given explicit output format.
func newTestConfig(format string) *app.Config {
	return &app.Config{Format: format, Fs: afero.NewMemMapFs()}
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
	return build(newTestConfig(format), fakeNewSvc(svc))
}

// runCmd executes a leaf cmd with its positional args and flags, returning
// everything it wrote. args must NOT include the leaf's own name: SetArgs
// feeds a single command, not a command path.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return buf.String()
}

// runCmdErr executes cmd expecting failure, returning the error and output.
func runCmdErr(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	require.Error(t, err)
	return buf.String(), err
}

// decodeJSON unmarshals one JSON document.
func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	return v
}

// jsonKeys returns the keys of a decoded JSON object.
func jsonKeys(t *testing.T, raw map[string]any) []string {
	t.Helper()
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	return keys
}

// requireSnakeCase asserts every key is lower snake_case, the output casing
// contract for JSON and TOON.
func requireSnakeCase(t *testing.T, keys []string) {
	t.Helper()
	for _, k := range keys {
		require.Regexp(t, `^[a-z0-9_]+$`, k, "key %q must be lower snake_case", k)
	}
}

// seedEntries returns a small realistic calendar list.
func seedEntries() []*calendar.CalendarListEntry {
	return []*calendar.CalendarListEntry{
		{Id: "oskar@example.com", Summary: "oskar@example.com", TimeZone: "Europe/Stockholm", Primary: true, ColorId: "tomato", Description: "Work calendar"},
		{Id: "abc123.group.calendar.google.com", Summary: "Team PTO", TimeZone: "Europe/Stockholm", ColorId: "banana"},
	}
}
