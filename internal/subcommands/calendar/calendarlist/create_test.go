package calendarlist

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestCreateMinimal(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "Team PTO")

	require.NotNil(t, svc.inserted)
	require.Equal(t, "Team PTO", svc.inserted.Summary)
	require.Empty(t, svc.inserted.Description)
	require.Empty(t, svc.inserted.TimeZone)
	// No --color-id, so the calendar list entry is never patched.
	require.Empty(t, svc.patchEntryID, "no color means no PatchCalendarList call")

	// The created calendar is echoed as output.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cal_99", row["id"])
	require.Equal(t, "Team PTO", row["summary"])
	require.EqualValues(t, "", row["color_id"])
}

func TestCreateFull(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"Team PTO",
		"--timezone", "Europe/Stockholm",
		"--description", "Shared time off",
		"--color-id", "tomato",
	)

	require.NotNil(t, svc.inserted)
	require.Equal(t, &calendar.Calendar{
		Summary:     "Team PTO",
		Description: "Shared time off",
		TimeZone:    "Europe/Stockholm",
	}, svc.inserted)
	// The color is patched onto the created calendar's list entry.
	require.Equal(t, "cal_99", svc.patchEntryID)
	require.NotNil(t, svc.patchEntry)
	require.Equal(t, "tomato", svc.patchEntry.ColorId)

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cal_99", row["id"])
	require.Equal(t, "Shared time off", row["description"])
	require.Equal(t, "Europe/Stockholm", row["timezone"])
	require.Equal(t, "tomato", row["color_id"])
}

func TestCreatePropagatesAPIError(t *testing.T) {
	svc := &fakeService{insertErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "Team PTO")

	require.Contains(t, err.Error(), "googleapi: Error 400")
	require.Nil(t, svc.patchEntry, "a failed insert must not patch the list entry")
}
