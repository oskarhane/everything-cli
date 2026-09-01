package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, []string{"id", "thread_id", "snippet", "label_ids"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "msg_1", first["id"])
	require.Equal(t, "thread_1", first["thread_id"])
	require.Equal(t, "Invoice attached", first["snippet"])
	require.Equal(t, []any{"INBOX", "UNREAD"}, first["label_ids"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"))

	// go-pretty StyleLight upper-cases headers; label_ids renders compactly.
	for _, header := range []string{"ID", "THREAD_ID", "SNIPPET", "LABEL_IDS"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "msg_1")
	require.Contains(t, out, "INBOX,UNREAD")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
}

func TestListComposesQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"query only", []string{"--query", "from:boss@corp.example"}, "from:boss@corp.example"},
		{"query shorthand", []string{"-q", "subject:invoice"}, "subject:invoice"},
		{"labels only", []string{"--label-ids", "INBOX, Label_7"}, "label:INBOX label:Label_7"},
		{"unread only", []string{"--unread-only"}, "is:unread"},
		{"all combined", []string{"-q", "after:2026/01/01", "--label-ids", "INBOX", "--unread-only"},
			"after:2026/01/01 label:INBOX is:unread"},
		{"no filters", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{messages: seedMessages()}
			cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), tt.args...)
			require.Equal(t, tt.want, svc.listQ)
		})
	}
}

func TestListHonorsMax(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--max", "1")

	require.EqualValues(t, 1, svc.listMax)
	// A single row renders as one JSON object, not a one-element array.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	require.Equal(t, "msg_1", row["id"])
}

func TestListDefaultMax(t *testing.T) {
	svc := &fakeService{messages: seedMessages()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.EqualValues(t, 25, svc.listMax)
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403: access denied")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
