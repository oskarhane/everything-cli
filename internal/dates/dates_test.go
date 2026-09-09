package dates

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fixedNow anchors relative parsing so expectations are deterministic.
var fixedNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// testZone is a non-UTC location so tests prove naive values adopt the
// pinned now's offset rather than assuming UTC. Fixed zone keeps it hermetic.
var testZone = time.FixedZone("TEST", 2*3600)

// nonUTCNow anchors naive parsing; its location must show up in output.
var nonUTCNow = time.Date(2026, 9, 1, 12, 0, 0, 0, testZone)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  EventTime
	}{
		{name: "RFC3339 UTC", value: "2026-09-03T14:00:00Z", want: EventTime{DateTime: "2026-09-03T14:00:00Z"}},
		{name: "RFC3339 offset", value: "2026-09-03T14:00:00+02:00", want: EventTime{DateTime: "2026-09-03T14:00:00+02:00"}},
		{name: "date only", value: "2026-09-03", want: EventTime{Date: "2026-09-03"}},
		{name: "now", value: "now", want: EventTime{DateTime: "2026-09-01T12:00:00Z"}},
		{name: "minus one day", value: "-1d", want: EventTime{DateTime: "2026-08-31T12:00:00Z"}},
		{name: "plus seven days", value: "+7d", want: EventTime{DateTime: "2026-09-08T12:00:00Z"}},
		{name: "bare seven days counts forward", value: "7d", want: EventTime{DateTime: "2026-09-08T12:00:00Z"}},
		{name: "minus thirty minutes", value: "-30m", want: EventTime{DateTime: "2026-09-01T11:30:00Z"}},
		{name: "plus two hours", value: "+2h", want: EventTime{DateTime: "2026-09-01T14:00:00Z"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.value, fixedNow)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseTimestampNaive(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  EventTime
	}{
		{name: "naive seconds", value: "2026-09-01T00:00:00", want: EventTime{DateTime: "2026-09-01T00:00:00+02:00"}},
		{name: "naive fractional seconds", value: "2026-09-01T00:00:00.5", want: EventTime{DateTime: "2026-09-01T00:00:00+02:00"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.value, nonUTCNow)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.True(t, strings.HasSuffix(got.DateTime, "+02:00"), "naive value must carry the local offset")
		})
	}
}

func TestParseTimestampErrorMentionsNaive(t *testing.T) {
	_, err := ParseTimestamp("tomorrow", nonUTCNow)
	require.Error(t, err)
	require.Contains(t, err.Error(), "naive")
	require.Contains(t, err.Error(), "local timezone")
}

func TestParseWindowTimeNaive(t *testing.T) {
	got, err := ParseWindowTime("2026-09-01T00:00:00", nonUTCNow)
	require.NoError(t, err)
	require.Equal(t, "2026-09-01T00:00:00+02:00", got)
}

func TestParseTimestampRejectsGarbage(t *testing.T) {
	for _, value := range []string{"", "tomorrow", "2026-9-3", "2026-02-30", "2026-09-03T14:00", "+7w", "d"} {
		_, err := ParseTimestamp(value, fixedNow)
		require.Error(t, err, "value %q must be rejected", value)
	}
}

func TestParseWindowTime(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "RFC3339 passthrough", value: "2026-09-03T14:00:00Z", want: "2026-09-03T14:00:00Z"},
		{name: "date means local midnight", value: "2026-09-03", want: "2026-09-03T00:00:00Z"},
		{name: "now", value: "now", want: "2026-09-01T12:00:00Z"},
		{name: "relative plus seven days", value: "+7d", want: "2026-09-08T12:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseWindowTime(tt.value, fixedNow)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseWindowTimeRejectsEmpty(t *testing.T) {
	_, err := ParseWindowTime("", fixedNow)
	require.Error(t, err, "empty bounds are unbounded only where callers pre-check")
}
