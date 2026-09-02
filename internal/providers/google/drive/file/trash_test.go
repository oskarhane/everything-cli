package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestTrashRecordsIDAndEchoes(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newTrashCmd, svc, "json"), "file_1")

	require.Equal(t, "file_1", svc.trashedID)
	require.Contains(t, out, "Trashed file file_1")
}

func TestTrashPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403: access denied")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTrashCmd, svc, "json"), "file_1")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestTrashRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTrashCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
