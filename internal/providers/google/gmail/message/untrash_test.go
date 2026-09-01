package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestUntrash(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newUntrashCmd, svc, "json"), "msg_1")

	require.Equal(t, "msg_1", svc.untrashedID)
	require.Equal(t, "Untrashed message msg_1\n", out, "single-line confirmation")
}

func TestUntrashPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUntrashCmd, svc, "json"), "msg_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
	require.Empty(t, svc.untrashedID)
}
