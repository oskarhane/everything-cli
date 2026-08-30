package event

import (
	"strings"

	calendar "google.golang.org/api/calendar/v3"
)

// Instance ids are `<masterId>_<RFC3339 UTC of the original start time>`
// (e.g. `kq3abc123_20260929T030000Z`); master ids are opaque and never
// contain `_`. The authoritative check is on the event resource
// (recurringEventId set => instance, recurrence set => master); the id shape
// is the heuristic when the resource has not been fetched. Never construct
// an instance id — resolve one via `event instances` or `event list`.

// isInstanceID reports whether eventID looks like a recurring-event instance
// id (contains `_`).
func isInstanceID(eventID string) bool {
	return strings.Contains(eventID, "_")
}

// masterID resolves the master id of a recurring series. It prefers the
// event resource's recurringEventId (authoritative) and falls back to the id
// shape heuristic (everything before the last `_`). For master and single
// event ids it returns eventID unchanged.
func masterID(ev *calendar.Event, eventID string) string {
	if ev != nil && ev.RecurringEventId != "" {
		return ev.RecurringEventId
	}
	if i := strings.LastIndex(eventID, "_"); i > 0 {
		return eventID[:i]
	}
	return eventID
}

// isRecurring reports whether ev belongs to a recurring series: a master has
// recurrence lines, an instance (or exception) carries a recurringEventId.
func isRecurring(ev *calendar.Event) bool {
	return ev.Recurrence != nil || ev.RecurringEventId != ""
}
