package thread

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)
	row, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"id", "snippet", "messages_count"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "thread_1", row["id"])
	require.Equal(t, "Invoice attached", row["snippet"])
	require.Equal(t, float64(1), row["messages_count"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"))

	for _, header := range []string{"ID", "SNIPPET", "MESSAGES_COUNT"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "thread_1")
	require.Contains(t, out, "Invoice attached")
}

func TestListPassesQueryAndMax(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"),
		"--query", "subject:invoice", "--label-ids", "Label_7, Label_8", "--max", "10")

	require.Equal(t, "subject:invoice", svc.listQ)
	require.Equal(t, []string{"Label_7", "Label_8"}, svc.listLabels)
	require.Equal(t, int64(10), svc.listMax)
}

func TestListDefaultsMaxTo25(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, int64(25), svc.listMax)
}

func TestListHonorsMax(t *testing.T) {
	svc := &fakeService{threads: seedThreads()}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--max", "1")

	require.EqualValues(t, 1, svc.listMax)
	// A single row renders as one JSON object, not a one-element array.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	require.Equal(t, "thread_1", row["id"])
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	require.Equal(t, "[]\n", out, "no threads renders an empty JSON array")
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "googleapi: Error 500")
}
