package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"
)

func TestMarkRead(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--read")

	require.Equal(t, "msg_1", svc.modifiedID)
	require.Equal(t, &gmail.ModifyMessageRequest{RemoveLabelIds: []string{"UNSEEN"}}, svc.modified)
}

func TestMarkUnread(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--unread")

	require.Equal(t, &gmail.ModifyMessageRequest{AddLabelIds: []string{"UNSEEN"}}, svc.modified)
}

func TestMarkStarred(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--starred")

	require.Equal(t, &gmail.ModifyMessageRequest{AddLabelIds: []string{"STARRED"}}, svc.modified)
}

func TestMarkUnstarred(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--unstarred")

	require.Equal(t, &gmail.ModifyMessageRequest{RemoveLabelIds: []string{"STARRED"}}, svc.modified)
}

func TestMarkReadAndStarred(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--read", "--starred")

	require.Equal(t, &gmail.ModifyMessageRequest{
		AddLabelIds:    []string{"STARRED"},
		RemoveLabelIds: []string{"UNSEEN"},
	}, svc.modified)
}

func TestMarkUnreadAndUnstarred(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--unread", "--unstarred")

	require.Equal(t, &gmail.ModifyMessageRequest{
		AddLabelIds:    []string{"UNSEEN"},
		RemoveLabelIds: []string{"STARRED"},
	}, svc.modified)
}

func TestMarkRequiresAtLeastOneFlag(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1")

	require.Contains(t, err.Error(), "nothing to mark")
	require.Nil(t, svc.modified, "empty mark must not reach the API")
}

func TestMarkRejectsReadAndUnread(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--read", "--unread")

	require.Contains(t, err.Error(), "--read and --unread are mutually exclusive")
	require.Nil(t, svc.modified)
}

func TestMarkRejectsStarredAndUnstarred(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--starred", "--unstarred")

	require.Contains(t, err.Error(), "--starred and --unstarred are mutually exclusive")
	require.Nil(t, svc.modified)
}

func TestMarkEchoesUpdatedMessage(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--read")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "msg_1", row["id"])
	require.Equal(t, []any{"INBOX", "STARRED"}, row["label_ids"])
}

func TestMarkPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 400")}
	_, err := runCmdErr(t, newLeafCmd(newMarkCmd, svc, "json"), "msg_1", "--read")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
