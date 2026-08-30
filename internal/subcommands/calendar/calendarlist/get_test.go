package calendarlist

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"
)

// seedCalendar returns the Calendar resource the primary calendar's
// calendars.get would serve.
func seedCalendar() *calendar.Calendar {
	return &calendar.Calendar{
		Id:          "oskar@example.com",
		Summary:     "oskar@example.com",
		Description: "Work calendar",
		TimeZone:    "Europe/Stockholm",
	}
}

func TestGetJSON(t *testing.T) {
	svc := &fakeService{entries: seedEntries(), getCal: seedCalendar()}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "json"), "oskar@example.com")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := jsonKeys(t, row)
	require.ElementsMatch(t, []string{"id", "summary", "description", "timezone", "color_id"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "oskar@example.com", row["id"])
	require.Equal(t, "Work calendar", row["description"])
	// color_id comes from the calendar list entry, not the Calendar resource.
	require.Equal(t, "tomato", row["color_id"])
}

func TestGetTable(t *testing.T) {
	svc := &fakeService{entries: seedEntries(), getCal: seedCalendar()}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "table"), "oskar@example.com")

	for _, header := range []string{"ID", "SUMMARY", "DESCRIPTION", "TIMEZONE", "COLOR_ID"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "Work calendar")
	require.Contains(t, out, "tomato")
}

func TestGetCalendarAPIError(t *testing.T) {
	svc := &fakeService{entries: seedEntries(), getCalErr: errors.New("googleapi: Error 404")}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "primary")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestGetCalendarListAPIError(t *testing.T) {
	// The calendar resource fetch succeeds but the list entry fetch (the
	// only source of color_id) fails, so the command fails.
	svc := &fakeService{entries: seedEntries(), getCal: seedCalendar(), getEntryErr: errors.New("googleapi: Error 403")}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "oskar@example.com")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{entries: seedEntries(), getCal: seedCalendar()}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
