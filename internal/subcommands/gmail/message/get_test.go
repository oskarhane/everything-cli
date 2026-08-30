package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestGetJSON(t *testing.T) {
	svc := &fakeService{messages: []*gmail.Message{seedDetailMessage()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "msg_1")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t,
		[]string{"id", "thread_id", "snippet", "label_ids", "from", "subject", "date"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "msg_1", row["id"])
	require.Equal(t, "thread_1", row["thread_id"])
	// from/subject/date are parsed out of the payload headers.
	require.Equal(t, "boss@corp.example", row["from"])
	require.Equal(t, "Invoice", row["subject"])
	require.Equal(t, "Mon, 24 Aug 2026 09:00:00 +0000", row["date"])
	require.Equal(t, "full", svc.getFormat)
}

func TestGetTable(t *testing.T) {
	svc := &fakeService{messages: []*gmail.Message{seedDetailMessage()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "table"), "msg_1")

	for _, header := range []string{"ID", "THREAD_ID", "SNIPPET", "LABEL_IDS", "FROM", "SUBJECT", "DATE"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "boss@corp.example")
	require.Contains(t, out, "INBOX,UNREAD")
}

func TestGetRaw(t *testing.T) {
	svc := &fakeService{messages: []*gmail.Message{seedDetailMessage()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "msg_1", "--raw")

	require.Equal(t, "raw", svc.getFormat)
	require.Contains(t, out, "From: boss@corp.example")
	require.Contains(t, out, "Subject: Invoice")
	require.Contains(t, out, "Please review the invoice.")
	require.NotContains(t, out, `"id"`, "--raw prints plain text, not JSON")
}

func TestGetRawIsBase64Decoded(t *testing.T) {
	// The raw wire form is base64url; the leaf must decode it before display.
	svc := &fakeService{messages: []*gmail.Message{seedDetailMessage()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "msg_1", "--raw")

	require.NotContains(t, out, seedDetailMessage().Raw, "undecoded base64url leaked to output")
}

func TestGetMissingHeaders(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "msg_2")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Empty(t, row["from"], "no payload headers means empty header fields")
	require.Empty(t, row["subject"])
	require.Empty(t, row["date"])
}

func TestGetNotFound(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "msg_404")

	require.Contains(t, err.Error(), "message msg_404 not found")
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "msg_1")

	require.Contains(t, err.Error(), "googleapi: Error 500")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
