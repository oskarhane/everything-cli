package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/subcommands/calendar/dates"
)

// fixedNow anchors relative parsing so expectations are deterministic.
var fixedNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// TestParseWindowTimeUnboundedEmpty pins the event-tree semantics that the
// shared dates.ParseWindowTime deliberately does not have: an empty window
// bound stays empty (unbounded).
func TestParseWindowTimeUnboundedEmpty(t *testing.T) {
	got, err := parseWindowTime("", fixedNow)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestToEventDateTime(t *testing.T) {
	require.Equal(t,
		&calendar.EventDateTime{Date: "2026-10-01"},
		toEventDateTime(dates.EventTime{Date: "2026-10-01"}, "Europe/Stockholm"),
		"date-only values must use the Date field even when a timezone is given")
	require.Equal(t,
		&calendar.EventDateTime{DateTime: "2026-10-01T09:00:00Z", TimeZone: "Europe/Stockholm"},
		toEventDateTime(dates.EventTime{DateTime: "2026-10-01T09:00:00Z"}, "Europe/Stockholm"))
	require.Equal(t,
		&calendar.EventDateTime{DateTime: "2026-10-01T09:00:00Z"},
		toEventDateTime(dates.EventTime{DateTime: "2026-10-01T09:00:00Z"}, ""))
}
