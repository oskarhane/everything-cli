package draft

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)
	row, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"id", "message_id", "snippet"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "draft_1", row["id"])
	require.Equal(t, "msg_1", row["message_id"])
	require.Equal(t, "Invoice attached", row["snippet"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"))

	for _, header := range []string{"ID", "MESSAGE_ID", "SNIPPET"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "draft_1")
	require.Contains(t, out, "Lunch tomorrow?")
}

func TestListPassesMax(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--max", "10")

	require.Equal(t, int64(10), svc.listMax)
}

func TestListDefaultsMaxTo25(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, int64(25), svc.listMax)
}

func TestListHonorsMax(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--max", "1")

	require.EqualValues(t, 1, svc.listMax)
	// A single row renders as one JSON object, not a one-element array.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	require.Equal(t, "draft_1", row["id"])
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 500")
}
