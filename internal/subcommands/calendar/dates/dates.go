// Package dates parses calendar timestamp flag values: the RFC3339, date,
// and relative-offset forms shared by the calendar command tree (event and
// freebusy window bounds).
package dates

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

const dateOnlyLayout = "2006-01-02"

// naiveLayouts are datetime layouts without an offset. A naive value assumes
// the local timezone (the now.Location() anchor).
var naiveLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
}

// relativeRe matches relative offsets like -1d, +7d, -30m, +2h. A bare count
// (7d) counts forward, like +7d. "now" is handled before the regexp runs.
var relativeRe = regexp.MustCompile(`^([-+]?\d+)([dhm])$`)

// EventTime is a parsed timestamp flag value: exactly one of Date (a
// YYYY-MM-DD date, valid for all-day events and window bounds) or DateTime
// (RFC3339 with an offset, as the Calendar API requires) is set.
type EventTime struct {
	Date     string
	DateTime string
}

// ParseTimestamp parses one timestamp flag value. Accepted forms:
//   - RFC3339 with an offset: 2026-09-03T14:00:00Z, 2026-09-03T14:00:00+02:00
//   - naive datetime (assumes the local timezone): 2026-09-03T14:00:00,
//     2026-09-03T14:00:00.5
//   - date only: 2026-09-03 (meaningful with --all-day or as a window bound)
//   - relative: now, -1d, +7d, -30m, +2h, 7d
//
// now anchors relative values; callers inject it so tests are deterministic.
// An empty value is an error; date-only values do not error here — the caller
// decides whether a date is acceptable (create/update require --all-day;
// window bounds convert it).
func ParseTimestamp(value string, now time.Time) (EventTime, error) {
	if value == "now" {
		return EventTime{DateTime: now.Format(time.RFC3339)}, nil
	}
	if m := relativeRe.FindStringSubmatch(value); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return EventTime{}, fmt.Errorf("invalid relative time %q: %w", value, err)
		}
		return EventTime{DateTime: now.Add(time.Duration(n) * RelativeUnit(m[2])).Format(time.RFC3339)}, nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return EventTime{DateTime: value}, nil
	}
	for _, layout := range naiveLayouts {
		if t, err := time.ParseInLocation(layout, value, now.Location()); err == nil {
			return EventTime{DateTime: t.Format(time.RFC3339)}, nil
		}
	}
	if _, err := time.Parse(dateOnlyLayout, value); err == nil {
		return EventTime{Date: value}, nil
	}
	return EventTime{}, fmt.Errorf("invalid timestamp %q: expected RFC3339 (2026-09-03T14:00:00Z or naive 2026-09-03T14:00:00, which assumes the local timezone), a date (2026-09-03), or a relative offset (now, -1d, +7d, -30m, +2h)", value)
}

// RelativeUnit maps a relative-offset unit letter to its duration.
func RelativeUnit(unit string) time.Duration {
	switch unit {
	case "d":
		return 24 * time.Hour
	case "h":
		return time.Hour
	default: // "m"
		return time.Minute
	}
}

// ParseWindowTime parses a --from/--to window bound into the RFC3339 value
// events.list/events.instances and freebusy.query require. A date-only bound
// means local midnight of that date. An empty value is an error: callers that
// treat an empty bound as unbounded must pre-check (the event tree does).
func ParseWindowTime(value string, now time.Time) (string, error) {
	et, err := ParseTimestamp(value, now)
	if err != nil {
		return "", err
	}
	if et.DateTime != "" {
		return et.DateTime, nil
	}
	t, err := time.ParseInLocation(dateOnlyLayout, et.Date, now.Location())
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339), nil
}
