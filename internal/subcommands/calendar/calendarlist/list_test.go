package calendarlist

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{entries: seedEntries()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := decodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := jsonKeys(t, first)
	require.ElementsMatch(t, []string{"id", "summary", "timezone", "primary"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "oskar@example.com", first["id"])
	require.Equal(t, "oskar@example.com", first["summary"])
	require.Equal(t, "Europe/Stockholm", first["timezone"])
	require.Equal(t, true, first["primary"], "primary is a bool, not a string")
}

func TestListTable(t *testing.T) {
	svc := &fakeService{entries: seedEntries()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "table"))

	// go-pretty StyleLight upper-cases headers.
	for _, header := range []string{"ID", "SUMMARY", "TIMEZONE", "PRIMARY"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "oskar@example.com")
	require.Contains(t, out, "Team PTO")
	require.Contains(t, out, "true")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, []any{}, decodeJSON(t, out))
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeService{listErr: errors.New("googleapi: Error 403")}
	_, err := runCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
