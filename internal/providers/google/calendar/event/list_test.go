package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListDefaultsToNowPlusSevenDays(t *testing.T) {
	freezeNow(t)
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Len(t, svc.listParams, 1)
	p := svc.listParams[0]
	require.Equal(t, "2026-09-01T12:00:00Z", p.TimeMin, "default --from is now")
	require.Equal(t, "2026-09-08T12:00:00Z", p.TimeMax, "default --to is +7d")
}

func TestListInstancesExpandsSeriesByDefault(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Len(t, svc.listParams, 1, "instances mode makes one expanded list call")
	p := svc.listParams[0]
	require.True(t, p.SingleEvents, "expansion requires singleEvents=true")
	require.Equal(t, "startTime", p.OrderBy, "singleEvents=true requires orderBy=startTime")
	require.NotEmpty(t, p.TimeMin)
	require.NotEmpty(t, p.TimeMax, "expansion must always be bounded by timeMax")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
}

func TestListInstancesRowsCarryRecurringFields(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows := cmdtest.DecodeJSON(t, out).([]any)
	byID := make(map[string]map[string]any, len(rows))
	for _, r := range rows {
		row := r.(map[string]any)
		byID[row["id"].(string)] = row
	}
	// The expanded occurrence: recurring, pointing at its master.
	require.True(t, byID[instanceEventID]["recurring"].(bool))
	require.Equal(t, masterEventID, byID[instanceEventID]["recurring_event_id"])
	// The master as returned by the fake (also in items): recurring, no
	// recurring_event_id of its own.
	require.True(t, byID[masterEventID]["recurring"].(bool))
	require.EqualValues(t, "", byID[masterEventID]["recurring_event_id"])
	// The single event: not recurring.
	require.False(t, byID["sing111"]["recurring"].(bool))
}

func TestListJSONKeysAreSnakeCase(t *testing.T) {
	svc := &fakeEventService{items: appendCancelled(seedListEvents())}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows := cmdtest.DecodeJSON(t, out).([]any)
	first := rows[0].(map[string]any)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, []string{
		"id", "summary", "start", "end", "status", "self_response",
		"recurring", "recurring_event_id", "organizer", "created", "updated", "description",
	}, keys)
	cmdtest.RequireSnakeCase(t, keys)

	byID := make(map[string]map[string]any, len(rows))
	for _, r := range rows {
		row := r.(map[string]any)
		byID[row["id"].(string)] = row
	}
	require.Equal(t, "cancelled", byID[cancelledEventID]["status"],
		"the seeded cancelled instance reports status=cancelled")
}

func TestListMastersReturnsDefaultList(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--recurring", "masters")

	require.Len(t, svc.listParams, 1)
	p := svc.listParams[0]
	require.False(t, p.SingleEvents, "masters mode is the default list call")
	require.Empty(t, p.OrderBy)
	require.NotEmpty(t, p.TimeMax, "the window bound applies to masters mode too")
}

func TestListAllMergesInstancesAndMasters(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--recurring", "all")

	require.Len(t, svc.listParams, 2, "all mode makes one expanded call and one masters call")
	require.True(t, svc.listParams[0].SingleEvents)
	require.Equal(t, "startTime", svc.listParams[0].OrderBy)
	require.False(t, svc.listParams[1].SingleEvents)

	// Both fake calls return the same three events; the merge must dedupe by
	// id, so the output still has exactly three rows.
	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
}

func TestListMaxDefaultsToCap(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Len(t, svc.listParams, 1)
	require.EqualValues(t, defaultListMax, svc.listParams[0].MaxResults,
		"default --max is the total cap (250)")
}

func TestListFromToForwarded(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"),
		"--from", "2026-09-01T00:00:00Z",
		"--to", "2026-09-08T00:00:00Z",
		"--query", "design review",
		"--max", "10")

	p := svc.listParams[0]
	require.Equal(t, "2026-09-01T00:00:00Z", p.TimeMin)
	require.Equal(t, "2026-09-08T00:00:00Z", p.TimeMax)
	require.Equal(t, "design review", p.Query)
	require.EqualValues(t, 10, p.MaxResults)
	require.Equal(t, "primary", p.CalendarID)
}

func TestListTableUpperCasedHeaders(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"))

	// go-pretty StyleLight upper-cases the snake_case field names.
	for _, header := range []string{"ID", "SUMMARY", "START", "END", "RECURRING", "RECURRING_EVENT_ID"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "Weekly standup")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeEventService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
}

func TestListShowDeletedAndUpdatedSinceDefaults(t *testing.T) {
	freezeNow(t)
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Len(t, svc.listParams, 1)
	require.True(t, svc.listParams[0].ShowDeleted, "default --show-deleted is true")
	require.Empty(t, svc.listParams[0].UpdatedMin, "empty --updated-since stays unset")
}

func TestListShowDeletedFalsePropagates(t *testing.T) {
	for _, mode := range []string{"instances", "masters", "all"} {
		svc := &fakeEventService{items: seedListEvents()}
		cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"),
			"--recurring", mode, "--show-deleted=false")

		require.NotEmpty(t, svc.listParams, mode)
		for _, p := range svc.listParams {
			require.False(t, p.ShowDeleted, "%s mode must carry --show-deleted=false", mode)
		}
	}
}

func TestListUpdatedSinceRelative(t *testing.T) {
	freezeNow(t)
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--updated-since", "-1d")

	p := svc.listParams[0]
	require.Equal(t, "2026-08-31T12:00:00Z", p.UpdatedMin,
		"--updated-since -1d resolves against the pinned now")
	require.True(t, p.ShowDeleted, "--show-deleted default carries alongside --updated-since")
}

func TestListUpdatedSinceExplicit(t *testing.T) {
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"),
		"--updated-since", "2026-09-01T00:00:00Z")

	require.Equal(t, "2026-09-01T00:00:00Z", svc.listParams[0].UpdatedMin)
}

func TestListAllModeCarriesShowDeletedAndUpdatedMin(t *testing.T) {
	freezeNow(t)
	svc := &fakeEventService{items: seedListEvents()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"),
		"--recurring", "all", "--updated-since", "-1d", "--show-deleted=false")

	require.Len(t, svc.listParams, 2)
	for i, p := range svc.listParams {
		require.False(t, p.ShowDeleted, "call %d carries --show-deleted=false", i)
		require.Equal(t, "2026-08-31T12:00:00Z", p.UpdatedMin, "call %d carries --updated-since", i)
	}
}

func TestListInvalidRecurringMode(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"), "--recurring", "sometimes")

	require.Contains(t, err.Error(), "invalid --recurring")
	require.Empty(t, svc.listParams, "no call must be made for a rejected mode")
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{listErr: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 404")
}
