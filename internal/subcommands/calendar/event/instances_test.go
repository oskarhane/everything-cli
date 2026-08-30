package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstancesCallsInstancesEndpoint(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := runCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID,
		"--from", "now", "--to", "+14d")

	require.Len(t, svc.instancesParams, 1)
	p := svc.instancesParams[0]
	require.Equal(t, masterEventID, p.EventID)
	require.Equal(t, "primary", p.CalendarID)
	require.NotEmpty(t, p.TimeMin)
	require.NotEmpty(t, p.TimeMax)

	rows, ok := decodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
}

func TestInstancesUnboundedByDefault(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	runCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	p := svc.instancesParams[0]
	require.Empty(t, p.TimeMin, "no --from means unbounded")
	require.Empty(t, p.TimeMax, "no --to means unbounded")
}

func TestInstancesDateBoundsBecomeMidnight(t *testing.T) {
	svc := &fakeEventService{}
	runCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID,
		"--from", "2026-09-01", "--to", "2026-09-15")

	p := svc.instancesParams[0]
	require.Contains(t, p.TimeMin, "2026-09-01T00:00:00")
	require.Contains(t, p.TimeMax, "2026-09-15T00:00:00")
}

func TestInstancesJSONKeysAreSnakeCase(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := runCmd(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	rows := decodeJSON(t, out).([]any)
	first := rows[0].(map[string]any)
	keys := jsonKeys(t, first)
	require.ElementsMatch(t, []string{"id", "summary", "start", "end", "recurring", "recurring_event_id"}, keys)
	requireSnakeCase(t, keys)
}

func TestInstancesPropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{instancesErr: errors.New("googleapi: Error 404")}
	_, err := runCmdErr(t, newLeafCmd(newInstancesCmd, svc, "json"), masterEventID)

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestInstancesRequiresMasterIDArg(t *testing.T) {
	svc := &fakeEventService{}
	_, err := runCmdErr(t, newLeafCmd(newInstancesCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
