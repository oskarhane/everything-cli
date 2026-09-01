package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestTrash(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newTrashCmd, svc, "json"), "msg_1")

	require.Equal(t, "msg_1", svc.trashedID)
	require.Equal(t, "Trashed message msg_1\n", out, "single-line confirmation")
}

func TestTrashPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTrashCmd, svc, "json"), "msg_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
	require.Empty(t, svc.trashedID)
}
