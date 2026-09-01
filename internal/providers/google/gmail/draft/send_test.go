package draft

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestSendSendsStoredDraft(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newSendCmd, svc, "json"), "draft_1")

	require.Equal(t, "draft_1", svc.sentDraft, "send must pass the draft id to the API")
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"id", "thread_id", "snippet"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "msg_99", row["id"])
	require.Equal(t, "thread_99", row["thread_id"])
}

func TestSendTable(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newSendCmd, svc, "table"), "draft_1")

	for _, header := range []string{"ID", "THREAD_ID", "SNIPPET"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "msg_99")
}

func TestSendPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"), "draft_1")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}

func TestSendRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
