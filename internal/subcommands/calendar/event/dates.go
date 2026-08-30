package event

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	calendar "google.golang.org/api/calendar/v3"
)

const dateOnlyLayout = "2006-01-02"

// relativeRe matches relative offsets like -1d, +7d, -30m, +2h. A bare count
// (7d) counts forward, like +7d. "now" is handled before the regexp runs.
var relativeRe = regexp.MustCompile(`^([-+]?\d+)([dhm])$`)

// eventTime is a parsed timestamp flag value: exactly one of date (a
// YYYY-MM-DD date, valid for all-day events and window bounds) or dateTime
// (RFC3339 with an offset, as the Calendar API requires) is set.
type eventTime struct {
	date     string
	dateTime string
}

// parseTimestamp parses one timestamp flag value. Accepted forms:
//   - RFC3339 with an offset: 2026-09-03T14:00:00Z, 2026-09-03T14:00:00+02:00
//   - date only: 2026-09-03 (meaningful with --all-day or as a window bound)
//   - relative: now, -1d, +7d, -30m, +2h, 7d
//
// now anchors relative values; callers inject it so tests are deterministic.
// Date-only values do not error here: the caller decides whether a date is
// acceptable (create/update require --all-day; window bounds convert it).
func parseTimestamp(value string, now time.Time) (eventTime, error) {
	if value == "now" {
		return eventTime{dateTime: now.Format(time.RFC3339)}, nil
	}
	if m := relativeRe.FindStringSubmatch(value); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return eventTime{}, fmt.Errorf("invalid relative time %q: %w", value, err)
		}
		return eventTime{dateTime: now.Add(time.Duration(n) * relativeUnit(m[2])).Format(time.RFC3339)}, nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return eventTime{dateTime: value}, nil
	}
	if _, err := time.Parse(dateOnlyLayout, value); err == nil {
		return eventTime{date: value}, nil
	}
	return eventTime{}, fmt.Errorf("invalid timestamp %q: expected RFC3339 (2026-09-03T14:00:00Z), a date (2026-09-03), or a relative offset (now, -1d, +7d, -30m, +2h)", value)
}

// relativeUnit maps a relative-offset unit letter to its duration.
func relativeUnit(unit string) time.Duration {
	switch unit {
	case "d":
		return 24 * time.Hour
	case "h":
		return time.Hour
	default: // "m"
		return time.Minute
	}
}

// parseWindowTime parses a --from/--to window bound into the RFC3339 value
// events.list/events.instances require. A date-only bound means local
// midnight of that date; an empty value stays empty (unbounded).
func parseWindowTime(value string, now time.Time) (string, error) {
	if value == "" {
		return "", nil
	}
	et, err := parseTimestamp(value, now)
	if err != nil {
		return "", err
	}
	if et.dateTime != "" {
		return et.dateTime, nil
	}
	t, err := time.ParseInLocation(dateOnlyLayout, et.date, now.Location())
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339), nil
}

// toEventDateTime maps a parsed start/end onto the API type. A date-only
// value becomes EventDateTime.Date (all-day); otherwise DateTime, with the
// optional IANA timeZone. Recurring series require a timezone.
func (t eventTime) toEventDateTime(timeZone string) *calendar.EventDateTime {
	if t.date != "" {
		return &calendar.EventDateTime{Date: t.date}
	}
	return &calendar.EventDateTime{DateTime: t.dateTime, TimeZone: timeZone}
}
