package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestCreateForwardsRecurrenceVerbatim(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Standup",
		"--start", "2026-09-01T09:00:00+02:00",
		"--end", "2026-09-01T09:30:00+02:00",
		"--recurrence", "RRULE:FREQ=WEEKLY;COUNT=10",
		"--recurrence", "EXDATE:20261225T090000Z",
	)

	require.NotNil(t, svc.inserted)
	// Raw RRULE:/EXDATE: values must land in the recurrence lines unchanged.
	require.Equal(t, []string{"RRULE:FREQ=WEEKLY;COUNT=10", "EXDATE:20261225T090000Z"}, svc.inserted.Recurrence)
	require.Equal(t, "2026-09-01T09:00:00+02:00", svc.inserted.Start.DateTime)
	require.Equal(t, "2026-09-01T09:30:00+02:00", svc.inserted.End.DateTime)
	require.Equal(t, "primary", svc.insertCalID)
}

func TestCreateAllDayUsesDateFields(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Conference",
		"--start", "2026-10-01",
		"--end", "2026-10-03",
		"--all-day",
	)

	require.Equal(t, "2026-10-01", svc.inserted.Start.Date)
	require.Empty(t, svc.inserted.Start.DateTime)
	require.Equal(t, "2026-10-03", svc.inserted.End.Date)
	require.Empty(t, svc.inserted.End.DateTime)
}

func TestCreateDateWithoutAllDayIsRejected(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Conference",
		"--start", "2026-10-01",
		"--end", "2026-10-03",
	)

	require.Contains(t, err.Error(), "pass --all-day")
	require.Nil(t, svc.inserted)
}

func TestCreateAttendeesRepeatable(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Design review",
		"--start", "2026-09-03T14:00:00Z",
		"--end", "2026-09-03T15:00:00Z",
		"--attendee", "alice@example.com",
		"--attendee", "bob@example.com",
	)

	require.Len(t, svc.inserted.Attendees, 2)
	require.Equal(t, "alice@example.com", svc.inserted.Attendees[0].Email)
	require.Equal(t, "bob@example.com", svc.inserted.Attendees[1].Email)
	require.Equal(t, "all", svc.insertSend, "guests present means send updates")
}

func TestCreateWithoutAttendeesSendsNoUpdates(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Focus block",
		"--start", "2026-09-03T14:00:00Z",
		"--end", "2026-09-03T15:00:00Z",
	)

	require.Empty(t, svc.inserted.Attendees)
	require.Equal(t, "none", svc.insertSend)
}

func TestCreateReminderMinutes(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Focus block",
		"--start", "2026-09-03T14:00:00Z",
		"--end", "2026-09-03T15:00:00Z",
		"--reminder-minutes", "30",
	)

	require.False(t, svc.inserted.Reminders.UseDefault)
	require.Len(t, svc.inserted.Reminders.Overrides, 1)
	require.Equal(t, "popup", svc.inserted.Reminders.Overrides[0].Method)
	require.EqualValues(t, 30, svc.inserted.Reminders.Overrides[0].Minutes)
	require.Contains(t, svc.inserted.Reminders.ForceSendFields, "UseDefault", "omitempty false must be force-sent")
}

func TestCreateTimezoneForwarded(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Standup",
		"--start", "2026-09-01T09:00:00+02:00",
		"--end", "2026-09-01T09:30:00+02:00",
		"--timezone", "Europe/Stockholm",
		"--recurrence", "RRULE:FREQ=WEEKLY;COUNT=10",
	)

	require.Equal(t, "Europe/Stockholm", svc.inserted.Start.TimeZone)
	require.Equal(t, "Europe/Stockholm", svc.inserted.End.TimeZone)
}

func TestCreateJSONOutput(t *testing.T) {
	svc := &fakeEventService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Design review",
		"--start", "2026-09-03T14:00:00Z",
		"--end", "2026-09-03T15:00:00Z",
	)

	view, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	require.Equal(t, "created123", view["id"])
	require.Equal(t, "Design review", view["summary"])
	require.Equal(t, []any{}, view["recurrence"], "nil recurrence renders as an empty list")
}

func TestCreateRequiresSummary(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--start", "2026-09-03T14:00:00Z",
		"--end", "2026-09-03T15:00:00Z",
	)

	require.Contains(t, err.Error(), "--summary is required")
}

func TestCreateRequiresStartAndEnd(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "--summary", "Orphan")

	require.Contains(t, err.Error(), "--start and --end are required")
}

func TestCreateRejectsInvalidStart(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Broken",
		"--start", "tomorrow",
		"--end", "2026-09-03T15:00:00Z",
	)

	require.Contains(t, err.Error(), "--start:")
	require.Contains(t, err.Error(), "invalid timestamp")
}

func TestCreatePropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{insertErr: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--summary", "Design review",
		"--start", "2026-09-03T14:00:00Z",
		"--end", "2026-09-03T15:00:00Z",
	)

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
