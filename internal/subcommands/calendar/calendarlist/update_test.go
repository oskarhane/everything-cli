package calendarlist

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdatePartialRename(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), "cal_99", "--summary", "Team PTO 2026")

	require.Equal(t, "cal_99", svc.patchedCalID)
	require.NotNil(t, svc.patchedCal)
	require.Equal(t, "Team PTO 2026", svc.patchedCal.Summary)
	// Partial update: nothing else is sent on the Calendar resource.
	require.Empty(t, svc.patchedCal.Description)
	require.Empty(t, svc.patchedCal.TimeZone)
	require.Empty(t, svc.patchEntryID, "no --color-id means no PatchCalendarList call")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cal_99", row["id"])
	require.Equal(t, "Team PTO 2026", row["summary"])
}

func TestUpdateColorOnly(t *testing.T) {
	// colorId lives on the calendar list entry, so a color-only update
	// must not touch the Calendar resource at all.
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), "cal_99", "--color-id", "banana")

	require.Empty(t, svc.patchedCalID, "color-only update must not patch the Calendar resource")
	require.Equal(t, "cal_99", svc.patchEntryID)
	require.NotNil(t, svc.patchEntry)
	require.Equal(t, "banana", svc.patchEntry.ColorId)

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cal_99", row["id"])
	require.Equal(t, "banana", row["color_id"])
}

func TestUpdateAllFlags(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newUpdateCmd, svc, "json"),
		"cal_99",
		"--summary", "Team PTO 2026",
		"--description", "Shared time off",
		"--timezone", "Europe/Stockholm",
		"--color-id", "tomato",
	)

	require.Equal(t, "cal_99", svc.patchedCalID)
	require.NotNil(t, svc.patchedCal)
	require.Equal(t, "Team PTO 2026", svc.patchedCal.Summary)
	require.Equal(t, "Shared time off", svc.patchedCal.Description)
	require.Equal(t, "Europe/Stockholm", svc.patchedCal.TimeZone)
	require.Equal(t, "cal_99", svc.patchEntryID)
	require.Equal(t, "tomato", svc.patchEntry.ColorId)

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Europe/Stockholm", row["timezone"])
	require.Equal(t, "tomato", row["color_id"])
}

func TestUpdateNothing(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), "cal_99")

	require.Contains(t, err.Error(), "nothing to update")
	require.Empty(t, svc.patchedCalID, "empty update must not reach the API")
	require.Empty(t, svc.patchEntryID, "empty update must not reach the API")
}

func TestUpdatePropagatesAPIError(t *testing.T) {
	svc := &fakeService{patchCalErr: errors.New("googleapi: Error 400")}
	_, err := runCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), "cal_99", "--summary", "Nope")

	require.Contains(t, err.Error(), "googleapi: Error 400")
	require.Empty(t, svc.patchEntryID, "a failed Calendar patch must not patch the list entry")
}
