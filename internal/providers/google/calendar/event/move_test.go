package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestMoveCallsMoveEvent(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	out := cmdtest.RunCmd(t, newLeafCmd(newMoveCmd, svc, "json"), instanceEventID,
		"--calendar", "work@example.com",
		"--to-calendar", "work.group.calendar.google.com")

	require.Len(t, svc.moveCalls, 1)
	m := svc.moveCalls[0]
	require.Equal(t, "work@example.com", m.calendarID)
	require.Equal(t, instanceEventID, m.eventID, "moving an instance id moves only that occurrence")
	require.Equal(t, "work.group.calendar.google.com", m.destCalendarID)

	view := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.Equal(t, instanceEventID, view["id"])
}

func TestMoveRequiresToCalendar(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newMoveCmd, svc, "json"), masterEventID)

	require.Contains(t, err.Error(), "--to-calendar is required")
	require.Empty(t, svc.moveCalls)
}

func TestMovePropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{moveErr: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newMoveCmd, svc, "json"), masterEventID, "--to-calendar", "other")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestMoveRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newMoveCmd, svc, "json"), "--to-calendar", "other")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
