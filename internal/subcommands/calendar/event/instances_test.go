package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestInstancesCallsInstancesEndpoint(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID,
		"--from", "now", "--to", "+14d")

	require.Len(t, svc.instancesParams, 1)
	p := svc.instancesParams[0]
	require.Equal(t, masterEventID, p.EventID)
	require.Equal(t, "primary", p.CalendarID)
	require.NotEmpty(t, p.TimeMin)
	require.NotEmpty(t, p.TimeMax)

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
}

func TestInstancesDefaultsBoundedWindow(t *testing.T) {
	freezeNow(t)
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	require.Len(t, svc.instancesParams, 1)
	p := svc.instancesParams[0]
	require.Equal(t, "2026-09-01T12:00:00Z", p.TimeMin, "default --from is now")
	require.Equal(t, "2026-09-08T12:00:00Z", p.TimeMax, "default --to is +7d")
	require.True(t, p.ShowDeleted, "cancelled occurrences are included by default")
}

func TestInstancesExplicitWindowOverridesDefaults(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID,
		"--from", "2020-01-01", "--to", "2030-01-01")

	p := svc.instancesParams[0]
	require.Contains(t, p.TimeMin, "2020-01-01T00:00:00")
	require.Contains(t, p.TimeMax, "2030-01-01T00:00:00")
}

func TestInstancesShowDeletedFalse(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID,
		"--show-deleted=false")

	require.False(t, svc.instancesParams[0].ShowDeleted,
		"--show-deleted=false hides cancelled occurrences")
}

func TestInstancesDateBoundsBecomeMidnight(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID,
		"--from", "2026-09-01", "--to", "2026-09-15")

	p := svc.instancesParams[0]
	require.Contains(t, p.TimeMin, "2026-09-01T00:00:00")
	require.Contains(t, p.TimeMax, "2026-09-15T00:00:00")
}

func TestInstancesJSONKeysAreSnakeCase(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	rows := cmdtest.DecodeJSON(t, out).([]any)
	first := rows[0].(map[string]any)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, []string{
		"id", "summary", "start", "end", "status", "self_response",
		"recurring", "recurring_event_id", "organizer", "created", "updated",
		"description",
	}, keys)
	cmdtest.RequireSnakeCase(t, keys)
}

func TestInstancesPropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{instancesErr: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestInstancesMaxForwardedAsTotalCap(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID, "--max", "7")

	p := svc.instancesParams[0]
	require.EqualValues(t, 7, p.MaxResults, "--max is the total cap")
}

func TestInstancesMaxDefaultsToCap(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	require.EqualValues(t, defaultListMax, svc.instancesParams[0].MaxResults,
		"default --max is the total cap (250)")
}

func TestInstancesRequiresMasterIDArg(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInstancesCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
