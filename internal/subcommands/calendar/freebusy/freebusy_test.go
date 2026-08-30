package freebusy

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"
)

// seedCalendars returns two calendar-list entries for the fallback path.
func seedCalendars() []*calendar.CalendarListEntry {
	return []*calendar.CalendarListEntry{
		{Id: "primary", Summary: "Personal"},
		{Id: "work@example.com", Summary: "Work"},
	}
}

// seedResponse returns a freebusy response with two busy periods on each of
// two calendars, covering every row shape `calendar freebusy` can emit.
func seedResponse() *calendar.FreeBusyResponse {
	return &calendar.FreeBusyResponse{
		Calendars: map[string]calendar.FreeBusyCalendar{
			"work@example.com": {Busy: []*calendar.TimePeriod{
				{Start: "2026-09-01T13:00:00Z", End: "2026-09-01T14:00:00Z"},
			}},
			"primary": {Busy: []*calendar.TimePeriod{
				{Start: "2026-09-01T15:00:00Z", End: "2026-09-01T15:30:00Z"},
				{Start: "2026-09-01T16:00:00Z", End: "2026-09-01T16:30:00Z"},
			}},
		},
	}
}

func TestFreeBusyDefaultsToNowPlusOneDay(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{
		entries: seedCalendars(),
		resp:    &calendar.FreeBusyResponse{Calendars: map[string]calendar.FreeBusyCalendar{}},
	}
	runCmd(t, newCmd(svc, "json"))

	require.True(t, svc.listCalled, "without --calendar the whole calendar list is queried")
	require.Len(t, svc.params, 1)
	p := svc.params[0]
	require.Equal(t, "2026-09-01T12:00:00Z", p.TimeMin, "default --from is now")
	require.Equal(t, "2026-09-02T12:00:00Z", p.TimeMax, "default --to is +1d")
	require.Equal(t, []string{"primary", "work@example.com"}, p.CalendarIDs,
		"default calendars come from the calendar list")
}

func TestFreeBusyExplicitFromToForwarded(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{
		entries: seedCalendars(),
		resp:    &calendar.FreeBusyResponse{Calendars: map[string]calendar.FreeBusyCalendar{}},
	}
	runCmd(t, newCmd(svc, "json"),
		"--from", "2026-09-01T09:00:00Z",
		"--to", "2026-09-01T17:00:00Z")

	p := svc.params[0]
	require.Equal(t, "2026-09-01T09:00:00Z", p.TimeMin)
	require.Equal(t, "2026-09-01T17:00:00Z", p.TimeMax)
}

func TestFreeBusyCalendarFlagOverridesCalendarList(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{entries: seedCalendars()}
	runCmd(t, newCmd(svc, "json"),
		"--calendar", "work@example.com",
		"--calendar", "team@example.com")

	require.False(t, svc.listCalled, "explicit --calendar values must skip the calendar list")
	require.Equal(t, []string{"work@example.com", "team@example.com"}, svc.params[0].CalendarIDs)
}

func TestFreeBusyCalendarFlagAcceptsCommaList(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{}
	runCmd(t, newCmd(svc, "json"), "--calendar", "a@example.com,b@example.com")

	require.Equal(t, []string{"a@example.com", "b@example.com"}, svc.params[0].CalendarIDs)
}

func TestFreeBusyJSONRowsSnakeCase(t *testing.T) {
	svc := &fakeFreeBusyService{entries: seedCalendars(), resp: seedResponse()}
	out := runCmd(t, newCmd(svc, "json"))

	rows, ok := decodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3, "one row per busy period across all calendars")
	first := rows[0].(map[string]any)
	keys := jsonKeys(t, first)
	require.ElementsMatch(t, []string{"calendar_id", "start", "end"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "primary", rows[0].(map[string]any)["calendar_id"])
	require.Equal(t, "2026-09-01T15:00:00Z", rows[0].(map[string]any)["start"])
	require.Equal(t, "2026-09-01T15:30:00Z", rows[0].(map[string]any)["end"])
}

func TestFreeBusyRowsOrderedByCalendarThenStart(t *testing.T) {
	svc := &fakeFreeBusyService{entries: seedCalendars(), resp: seedResponse()}
	out := runCmd(t, newCmd(svc, "json"))

	rows := decodeJSON(t, out).([]any)
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		row := r.(map[string]any)
		got = append(got, row["calendar_id"].(string)+"@"+row["start"].(string))
	}
	require.Equal(t, []string{
		"primary@2026-09-01T15:00:00Z",
		"primary@2026-09-01T16:00:00Z",
		"work@example.com@2026-09-01T13:00:00Z",
	}, got)
}

func TestFreeBusyTableUpperCasedHeaders(t *testing.T) {
	svc := &fakeFreeBusyService{entries: seedCalendars(), resp: seedResponse()}
	out := runCmd(t, newCmd(svc, "table"))

	// go-pretty StyleLight upper-cases the snake_case field names.
	for _, header := range []string{"CALENDAR_ID", "START", "END"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "2026-09-01T15:00:00Z")
}

func TestFreeBusyEmptyResultRendersCleanly(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{
		entries: seedCalendars(),
		resp:    &calendar.FreeBusyResponse{Calendars: map[string]calendar.FreeBusyCalendar{}},
	}
	out := runCmd(t, newCmd(svc, "json"))

	require.Equal(t, []any{}, decodeJSON(t, out))
}

func TestFreeBusyPropagatesQueryError(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{
		entries:  seedCalendars(),
		queryErr: errors.New("googleapi: Error 400"),
	}
	_, err := runCmdErr(t, newCmd(svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 400")
}

func TestFreeBusyPropagatesCalendarListError(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{listErr: errors.New("googleapi: Error 403")}
	_, err := runCmdErr(t, newCmd(svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 403")
	require.Empty(t, svc.params, "no query must run when the calendar list fails")
}

func TestFreeBusyInvalidWindow(t *testing.T) {
	freezeNow(t)
	svc := &fakeFreeBusyService{entries: seedCalendars()}
	_, err := runCmdErr(t, newCmd(svc, "json"), "--from", "yesterday")

	require.Contains(t, err.Error(), "invalid timestamp")
	require.Empty(t, svc.params, "no query must run for a rejected window")
}

// TestExamplesGate enforces the example contract: flush-left comment-led
// example with at least two google-cli invocations and --format json.
func TestExamplesGate(t *testing.T) {
	cmd := newCmd(&fakeFreeBusyService{}, "json")
	leaf := cmd
	require.Equal(t, "freebusy", leaf.Name())

	example := leaf.Example
	require.NotEmpty(t, example, "the leaf needs an Example")
	require.True(t, strings.HasPrefix(example, "# "), "Example must be flush-left, starting with a # comment")
	require.GreaterOrEqual(t, strings.Count(example, "google-cli calendar freebusy"), 2,
		"Example needs at least two google-cli invocations")
	require.Contains(t, example, "# ", "Example needs # comments")
	require.Contains(t, example, "--format json", "Example needs a --format json call")
}
