package label

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetByID(t *testing.T) {
	svc := &fakeService{labels: seedLabels()}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "json"), "INBOX")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := jsonKeys(t, row)
	require.ElementsMatch(t, []string{"id", "name", "type", "unread_count", "messages_total", "threads_total"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "INBOX", row["id"])
	require.Equal(t, "system", row["type"])
}

func TestGetByNameFallsBackToList(t *testing.T) {
	// The direct id lookup fails (a name is not an id), so get resolves by
	// listing and matching label names.
	svc := &fakeService{labels: seedLabels(), getErr: errors.New("googleapi: Error 404")}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "json"), "Travel")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Label_7", row["id"])
	require.Equal(t, "Travel", row["name"])
}

func TestGetByNameTable(t *testing.T) {
	svc := &fakeService{labels: seedLabels(), getErr: errors.New("googleapi: Error 404")}
	out := runCmd(t, newLeafCmd(newGetCmd, svc, "table"), "Travel")

	for _, header := range []string{"ID", "NAME", "TYPE"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "Travel")
}

func TestGetNotFound(t *testing.T) {
	svc := &fakeService{labels: seedLabels(), getErr: errors.New("googleapi: Error 404")}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "Nope")

	require.Contains(t, err.Error(), `label "Nope" not found by id or name`)
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{labels: seedLabels()}
	_, err := runCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
