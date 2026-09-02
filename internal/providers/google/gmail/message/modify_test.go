package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestModifyAddAndRemove(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newModifyCmd, svc, "json"),
		"msg_1", "--add-label-ids", "Label_7, Label_9", "--remove-label-ids", "INBOX")

	require.Equal(t, "msg_1", svc.modifiedID)
	require.Equal(t, &gmail.ModifyMessageRequest{
		AddLabelIds:    []string{"Label_7", "Label_9"},
		RemoveLabelIds: []string{"INBOX"},
	}, svc.modified)
}

func TestModifyAddOnly(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newModifyCmd, svc, "json"), "msg_1", "--add-label-ids", "STARRED")

	require.Equal(t, &gmail.ModifyMessageRequest{AddLabelIds: []string{"STARRED"}}, svc.modified)
}

func TestModifyRequiresAtLeastOneList(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newModifyCmd, svc, "json"), "msg_1")

	require.Contains(t, err.Error(), "nothing to modify")
	require.Nil(t, svc.modified, "empty modify must not reach the API")
}

func TestModifyRequiresAtLeastOneListWhenOnlySpaces(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newModifyCmd, svc, "json"),
		"msg_1", "--add-label-ids", " , ")

	require.Contains(t, err.Error(), "nothing to modify")
	require.Nil(t, svc.modified)
}

func TestModifyEchoesUpdatedMessage(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newModifyCmd, svc, "json"), "msg_1", "--add-label-ids", "STARRED")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, row)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "msg_1", row["id"])
	require.Equal(t, []any{"INBOX", "STARRED"}, row["label_ids"])
}

func TestModifyPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newModifyCmd, svc, "json"),
		"msg_1", "--add-label-ids", "STARRED")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
