package event

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestUpdateInstancePatchesOnlyThatOccurrence(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), instanceEventID, "--summary", "Standup moved")

	require.Len(t, svc.patches, 1)
	p := svc.patches[0]
	require.Equal(t, instanceEventID, p.eventID, "an instance id must be patched as itself, creating an exception")
	require.Equal(t, "primary", p.calendarID)
	require.Equal(t, "Standup moved", p.event.Summary)
	require.Equal(t, "all", p.sendUpdates)

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.Equal(t, "Standup moved", view["summary"])
}

func TestUpdateMasterIDPatchesTheSeries(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), masterEventID, "--summary", "Standup moved")

	require.Len(t, svc.patches, 1)
	require.Equal(t, masterEventID, svc.patches[0].eventID)
}

func TestUpdateThisOnlyFalseWithInstancePatchesMaster(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), instanceEventID,
		"--this-only=false", "--summary", "Standup moved")

	require.Len(t, svc.patches, 1)
	require.Equal(t, masterEventID, svc.patches[0].eventID, "--this-only=false resolves the instance id to its master")
}

func TestUpdateRequiresAtLeastOneChangeFlag(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), masterEventID)

	require.Contains(t, err.Error(), "nothing to update")
	require.Empty(t, svc.patches)
}

func TestUpdateAttendeeChangesEchoTheFullArray(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), masterEventID,
		"--remove-attendee", "organizer@example.com",
		"--add-attendee", "colleague@example.com",
	)

	require.Len(t, svc.patches, 1)
	patch := svc.patches[0].event
	require.Len(t, patch.Attendees, 2, "patch overwrites arrays wholesale, so the full list must be sent")
	require.Equal(t, "me@example.com", patch.Attendees[0].Email)
	require.Equal(t, "colleague@example.com", patch.Attendees[1].Email)
	// Only the attendee field is in the patch body.
	require.Empty(t, patch.Summary)
	require.Nil(t, patch.Start)
}

func TestUpdateDateOnAllDayEvent(t *testing.T) {
	svc := &fakeEventService{events: seedAllDayEvent()}
	cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), "allday1", "--start", "2026-10-02")

	patch := svc.patches[0].event
	require.Equal(t, "2026-10-02", patch.Start.Date, "all-day events keep the Date field")
	require.Empty(t, patch.Start.DateTime)
}

func TestUpdateDateOnTimedEventIsRejected(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), instanceEventID, "--start", "2026-10-02")

	require.Contains(t, err.Error(), "not all-day")
	require.Empty(t, svc.patches)
}

func TestUpdatePropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), "missing1", "--summary", "X")

	require.Contains(t, err.Error(), "404")
	require.Empty(t, svc.patches)
}
