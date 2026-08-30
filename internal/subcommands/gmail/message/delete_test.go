package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "msg_1")

	require.Contains(t, err.Error(), `refusing to permanently delete message "msg_1" without --force`)
	require.Contains(t, err.Error(), "cannot be undone", "the refusal must warn that deletion is permanent")
	require.False(t, svc.deleted, "permanent delete must not reach the API without --force")
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), "msg_1", "--force")

	require.True(t, svc.deleted)
	require.Equal(t, "msg_1", svc.deletedID)
	require.Empty(t, out, "successful delete is silent")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 400")}
	_, err := runCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "msg_1", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
