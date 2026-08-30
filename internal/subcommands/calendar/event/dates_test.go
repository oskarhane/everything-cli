package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"
)

// fixedNow anchors relative parsing so expectations are deterministic.
var fixedNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  eventTime
	}{
		{name: "RFC3339 UTC", value: "2026-09-03T14:00:00Z", want: eventTime{dateTime: "2026-09-03T14:00:00Z"}},
		{name: "RFC3339 offset", value: "2026-09-03T14:00:00+02:00", want: eventTime{dateTime: "2026-09-03T14:00:00+02:00"}},
		{name: "date only", value: "2026-09-03", want: eventTime{date: "2026-09-03"}},
		{name: "now", value: "now", want: eventTime{dateTime: "2026-09-01T12:00:00Z"}},
		{name: "minus one day", value: "-1d", want: eventTime{dateTime: "2026-08-31T12:00:00Z"}},
		{name: "plus seven days", value: "+7d", want: eventTime{dateTime: "2026-09-08T12:00:00Z"}},
		{name: "bare seven days counts forward", value: "7d", want: eventTime{dateTime: "2026-09-08T12:00:00Z"}},
		{name: "minus thirty minutes", value: "-30m", want: eventTime{dateTime: "2026-09-01T11:30:00Z"}},
		{name: "plus two hours", value: "+2h", want: eventTime{dateTime: "2026-09-01T14:00:00Z"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimestamp(tt.value, fixedNow)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseTimestampRejectsGarbage(t *testing.T) {
	for _, value := range []string{"tomorrow", "2026-9-3", "2026-02-30", "2026-09-03T14:00", "+7w", "d"} {
		_, err := parseTimestamp(value, fixedNow)
		require.Error(t, err, "value %q must be rejected", value)
	}
}

func TestParseWindowTime(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty stays empty (unbounded)", value: "", want: ""},
		{name: "RFC3339 passthrough", value: "2026-09-03T14:00:00Z", want: "2026-09-03T14:00:00Z"},
		{name: "date means local midnight", value: "2026-09-03", want: "2026-09-03T00:00:00Z"},
		{name: "now", value: "now", want: "2026-09-01T12:00:00Z"},
		{name: "relative plus seven days", value: "+7d", want: "2026-09-08T12:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWindowTime(tt.value, fixedNow)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEventTimeToEventDateTime(t *testing.T) {
	require.Equal(t,
		&calendar.EventDateTime{Date: "2026-10-01"},
		eventTime{date: "2026-10-01"}.toEventDateTime("Europe/Stockholm"),
		"date-only values must use the Date field even when a timezone is given")
	require.Equal(t,
		&calendar.EventDateTime{DateTime: "2026-10-01T09:00:00Z", TimeZone: "Europe/Stockholm"},
		eventTime{dateTime: "2026-10-01T09:00:00Z"}.toEventDateTime("Europe/Stockholm"))
	require.Equal(t,
		&calendar.EventDateTime{DateTime: "2026-10-01T09:00:00Z"},
		eventTime{dateTime: "2026-10-01T09:00:00Z"}.toEventDateTime(""))
}
