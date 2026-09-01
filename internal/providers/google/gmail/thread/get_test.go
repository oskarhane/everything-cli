package thread

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestGetJSON(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "thread_1")

	// A single message renders as one JSON object, not a one-element array.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"id", "from", "subject", "date", "snippet"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "msg_1", row["id"])
	// from/subject/date are parsed out of the payload headers.
	require.Equal(t, "boss@corp.example", row["from"])
	require.Equal(t, "Invoice", row["subject"])
	require.Equal(t, "Mon, 24 Aug 2026 09:00:00 +0000", row["date"])
	require.Equal(t, "thread_1", svc.getID)
}

func TestGetTable(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "thread_1")

	for _, header := range []string{"ID", "FROM", "SUBJECT", "DATE", "SNIPPET"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "boss@corp.example")
	require.Contains(t, out, "Invoice")
}

func TestGetMultiMessageThread(t *testing.T) {
	// A thread with replies shows every message, newest last.
	multi := &gmail.Thread{
		Id:      "thread_3",
		Snippet: "Invoice attached",
		Messages: []*gmail.Message{
			seedMessage("msg_1", "boss@corp.example", "Invoice", "Mon, 24 Aug 2026 09:00:00 +0000"),
			seedMessage("msg_3", "me@example.com", "Re: Invoice", "Mon, 24 Aug 2026 10:00:00 +0000"),
		},
	}
	svc := &fakeService{threads: []*gmail.Thread{multi}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "thread_3")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok)
	require.Len(t, rows, 2)
	first, ok := rows[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "msg_3", first["id"])
}

func TestGetNotFound(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "thread_404")

	require.Contains(t, err.Error(), "thread thread_404 not found")
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "thread_1")

	require.Contains(t, err.Error(), "googleapi: Error 500")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
