package event

import (
	"time"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/dates"
)

// parseWindowTime parses a --from/--to window bound into the RFC3339 value
// events.list/events.instances require, wrapping the shared dates parser.
// An empty bound stays empty (unbounded).
func parseWindowTime(value string, now time.Time) (string, error) {
	if value == "" {
		return "", nil
	}
	return dates.ParseWindowTime(value, now)
}

// toEventDateTime maps a parsed start/end onto the API type. A date-only
// value becomes EventDateTime.Date (all-day); otherwise DateTime, with the
// optional IANA timeZone. Recurring series require a timezone.
func toEventDateTime(t dates.EventTime, timeZone string) *calendar.EventDateTime {
	if t.Date != "" {
		return &calendar.EventDateTime{Date: t.Date}
	}
	return &calendar.EventDateTime{DateTime: t.DateTime, TimeZone: timeZone}
}
