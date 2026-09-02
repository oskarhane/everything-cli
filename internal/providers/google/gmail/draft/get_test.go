package draft

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestGetJSON(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "draft_1")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t,
		[]string{"id", "message_id", "from", "to", "subject", "date", "snippet"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "draft_1", row["id"])
	require.Equal(t, "msg_1", row["message_id"])
	// from/to/subject/date are parsed out of the stored message's headers.
	require.Equal(t, "me@example.com", row["from"])
	require.Equal(t, "boss@corp.example", row["to"])
	require.Equal(t, "Invoice", row["subject"])
	require.Equal(t, "Mon, 24 Aug 2026 09:00:00 +0000", row["date"])
	require.Equal(t, "draft_1", svc.getID)
}

func TestGetTable(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "draft_1")

	for _, header := range []string{"ID", "MESSAGE_ID", "FROM", "TO", "SUBJECT", "DATE", "SNIPPET"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "boss@corp.example")
}

func TestGetWithoutHeaders(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "draft_2")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "draft_2", row["id"])
	require.Empty(t, row["from"], "no payload headers means empty header fields")
	require.Empty(t, row["subject"])
}

func TestGetNotFound(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "draft_404")

	require.Contains(t, err.Error(), "draft draft_404 not found")
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "draft_1")

	require.Contains(t, err.Error(), "googleapi: Error 500")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{drafts: seedDrafts()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
