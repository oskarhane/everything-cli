package event

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestGetJSONMasterView(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), masterEventID)

	view, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, view)
	require.ElementsMatch(t, []string{
		"id", "summary", "start", "end", "status", "self_response",
		"created", "updated", "location", "meet_link", "description",
		"attendees", "recurring", "recurring_event_id", "recurrence",
	}, keys)
	cmdtest.RequireSnakeCase(t, keys)

	require.Equal(t, masterEventID, view["id"])
	require.True(t, view["recurring"].(bool))
	// The master carries the raw recurrence lines.
	require.Equal(t, []any{"RRULE:FREQ=WEEKLY;COUNT=10"}, view["recurrence"])

	// Attendees render with email and response_status keys.
	attendees := view["attendees"].([]any)
	require.Len(t, attendees, 2)
	first := attendees[0].(map[string]any)
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, first))
	require.Equal(t, "organizer@example.com", first["email"])
	require.Equal(t, "accepted", first["response_status"])
}

func TestGetInstanceView(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), instanceEventID)

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.Equal(t, instanceEventID, view["id"])
	require.True(t, view["recurring"].(bool))
	require.Equal(t, masterEventID, view["recurring_event_id"])
	require.Equal(t, []any{}, view["recurrence"], "instances have no recurrence lines of their own")
}

func TestGetTableUpperCasedHeaders(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), masterEventID)

	for _, header := range []string{"ID", "SUMMARY", "STATUS", "SELF_RESPONSE", "RECURRING", "RECURRING_EVENT_ID", "RECURRENCE", "ATTENDEES"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "RRULE:FREQ=WEEKLY;COUNT=10")
	require.Contains(t, out, "organizer@example.com (accepted)")
}

// seedStatusEvents returns a confirmed event whose acting-account attendee
// has accepted, and a cancelled event with no self attendee, covering both
// self_response shapes the get view can emit. The fields the series seeds
// leave zero-valued (status/created/updated/organizer) are set here, so no
// helpers_test.go edits are needed.
func seedGetEventShapes() map[string]*calendar.Event {
	return map[string]*calendar.Event{
		"getevt1": {
			Id:        "getevt1",
			Summary:   "Field check",
			Status:    "confirmed",
			Created:   "2026-08-01T08:00:00Z",
			Updated:   "2026-08-02T09:00:00Z",
			Organizer: &calendar.EventOrganizer{Email: "organizer@example.com"},
			Start:     &calendar.EventDateTime{DateTime: "2026-09-02T10:00:00Z"},
			End:       &calendar.EventDateTime{DateTime: "2026-09-02T10:30:00Z"},
			Attendees: []*calendar.EventAttendee{
				{Email: "organizer@example.com", ResponseStatus: "accepted", Organizer: true},
				{Email: "me@example.com", ResponseStatus: "accepted", Self: true},
			},
		},
		"cancel1": {
			Id:      "cancel1",
			Summary: "Dropped meeting",
			Status:  "cancelled",
			Created: "2026-08-03T08:00:00Z",
			Updated: "2026-08-03T08:30:00Z",
			Start:   &calendar.EventDateTime{DateTime: "2026-09-03T10:00:00Z"},
			End:     &calendar.EventDateTime{DateTime: "2026-09-03T10:30:00Z"},
			Attendees: []*calendar.EventAttendee{
				{Email: "other@example.com", ResponseStatus: "needsAction"},
			},
		},
	}
}

func TestGetJSONStatusAndSelfResponse(t *testing.T) {
	svc := &fakeEventService{events: seedGetEventShapes()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "getevt1")

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	keys := cmdtest.JSONKeys(t, view)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "confirmed", view["status"])
	require.Equal(t, "accepted", view["self_response"])
	require.Equal(t, "2026-08-01T08:00:00Z", view["created"])
	require.Equal(t, "2026-08-02T09:00:00Z", view["updated"])
}

func TestGetJSONCancelledEventWithoutSelfAttendee(t *testing.T) {
	svc := &fakeEventService{events: seedGetEventShapes()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "cancel1")

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	keys := cmdtest.JSONKeys(t, view)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "cancelled", view["status"])
	require.EqualValues(t, "", view["self_response"], "no self attendee means an empty self_response")
}

func TestGetTableStatusAndSelfResponseHeaders(t *testing.T) {
	svc := &fakeEventService{events: seedGetEventShapes()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "getevt1")

	// go-pretty StyleLight upper-cases the headers, not the cells.
	for _, header := range []string{"STATUS", "SELF_RESPONSE", "CREATED", "UPDATED"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "confirmed")
	require.Contains(t, out, "accepted")
}

func TestGetFetchesTheGivenIDOnce(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), masterEventID, "--calendar", "work@example.com")

	require.Equal(t, []string{masterEventID}, svc.getCalls)
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "missing1")

	require.Contains(t, err.Error(), "404")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}

// seedMeetLinkEvents returns one event with a Google Meet hangout link and
// one without, covering both meet_link shapes the get view can emit.
func seedMeetLinkEvents() map[string]*calendar.Event {
	return map[string]*calendar.Event{
		"meetlink1": {
			Id:          "meetlink1",
			Summary:     "Sync call",
			Location:    "Remote",
			HangoutLink: "https://meet.google.com/abc-mnop-xyz",
			Start:       &calendar.EventDateTime{DateTime: "2026-09-02T10:00:00Z"},
			End:         &calendar.EventDateTime{DateTime: "2026-09-02T10:30:00Z"},
		},
		"nomeet1": {
			Id:      "nomeet1",
			Summary: "Desk work",
			Start:   &calendar.EventDateTime{DateTime: "2026-09-02T12:00:00Z"},
			End:     &calendar.EventDateTime{DateTime: "2026-09-02T13:00:00Z"},
		},
	}
}

func TestGetJSONMeetLink(t *testing.T) {
	svc := &fakeEventService{events: seedMeetLinkEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "meetlink1")

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	keys := cmdtest.JSONKeys(t, view)
	cmdtest.RequireSnakeCase(t, keys)
	require.Contains(t, keys, "meet_link")
	require.Equal(t, "https://meet.google.com/abc-mnop-xyz", view["meet_link"])
}

func TestGetJSONMeetLinkEmptyWhenAbsent(t *testing.T) {
	svc := &fakeEventService{events: seedMeetLinkEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "nomeet1")

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	// An event with no hangout link still carries the key, as an empty string.
	require.Contains(t, cmdtest.JSONKeys(t, view), "meet_link")
	require.EqualValues(t, "", view["meet_link"])
}

func TestGetToonMeetLink(t *testing.T) {
	svc := &fakeEventService{events: seedMeetLinkEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "toon"), "meetlink1")

	// The URL contains ':', so TOON quotes the value.
	require.Contains(t, out, `meet_link: "https://meet.google.com/abc-mnop-xyz"`)
}

func TestGetTableMeetLink(t *testing.T) {
	svc := &fakeEventService{events: seedMeetLinkEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "meetlink1")

	// go-pretty StyleLight upper-cases the snake_case field name.
	require.Contains(t, out, "MEET_LINK")
	require.Contains(t, out, "https://meet.google.com/abc-mnop-xyz")
}

// TestEventViewFieldsPutMeetLinkAfterLocation pins the field-order contract:
// meet_link is a where/how-to-join field, so it sits directly after location
// in the single-event view.
func TestEventViewFieldsPutMeetLinkAfterLocation(t *testing.T) {
	i := slices.Index(eventViewFields, "location")
	require.GreaterOrEqual(t, i, 0, "eventViewFields must contain location")
	require.Less(t, i+1, len(eventViewFields), "location must not be the last field")
	require.Equal(t, "meet_link", eventViewFields[i+1])
}
