package event

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetJSONMasterView(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "json"), masterEventID)

	view, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := jsonKeys(t, view)
	require.ElementsMatch(t, []string{
		"id", "summary", "start", "end", "location", "description",
		"attendees", "recurring", "recurring_event_id", "recurrence",
	}, keys)
	requireSnakeCase(t, keys)

	require.Equal(t, masterEventID, view["id"])
	require.True(t, view["recurring"].(bool))
	// The master carries the raw recurrence lines.
	require.Equal(t, []any{"RRULE:FREQ=WEEKLY;COUNT=10"}, view["recurrence"])

	// Attendees render with email and response_status keys.
	attendees := view["attendees"].([]any)
	require.Len(t, attendees, 2)
	first := attendees[0].(map[string]any)
	requireSnakeCase(t, jsonKeys(t, first))
	require.Equal(t, "organizer@example.com", first["email"])
	require.Equal(t, "accepted", first["response_status"])
}

func TestGetInstanceView(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "json"), instanceEventID)

	view := decodeJSON(t, out).(map[string]any)
	require.Equal(t, instanceEventID, view["id"])
	require.True(t, view["recurring"].(bool))
	require.Equal(t, masterEventID, view["recurring_event_id"])
	require.Equal(t, []any{}, view["recurrence"], "instances have no recurrence lines of their own")
}

func TestGetTableUpperCasedHeaders(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "table"), masterEventID)

	for _, header := range []string{"ID", "SUMMARY", "RECURRING", "RECURRING_EVENT_ID", "RECURRENCE", "ATTENDEES"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "RRULE:FREQ=WEEKLY;COUNT=10")
	require.Contains(t, out, "organizer@example.com (accepted)")
}

func TestGetFetchesTheGivenIDOnce(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	runCmd(t, newLeafCmd(newGetCmd, svc, "json"), masterEventID, "--calendar", "work@example.com")

	require.Equal(t, []string{masterEventID}, svc.getCalls)
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "missing1")

	require.Contains(t, err.Error(), "404")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
