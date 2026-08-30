package label

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{labels: seedLabels()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := decodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := jsonKeys(t, first)
	require.ElementsMatch(t, []string{"id", "name", "type", "unread_count", "messages_total", "threads_total"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "INBOX", first["id"])
	require.Equal(t, "INBOX", first["name"])
	require.Equal(t, "system", first["type"])
	require.EqualValues(t, 3, first["unread_count"])
	require.EqualValues(t, 12, first["messages_total"])
	require.EqualValues(t, 9, first["threads_total"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{labels: seedLabels()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "table"))

	// go-pretty StyleLight upper-cases headers.
	for _, header := range []string{"ID", "NAME", "TYPE", "UNREAD_COUNT", "MESSAGES_TOTAL", "THREADS_TOTAL"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "INBOX")
	require.Contains(t, out, "Travel")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, []any{}, decodeJSON(t, out))
}
