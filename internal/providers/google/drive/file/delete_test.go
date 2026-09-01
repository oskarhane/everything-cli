package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "file_1")

	// The refusal wording is contractual: it names the file, the --force
	// remedy, and the recoverable alternative, and no service call may have
	// happened.
	require.ErrorContains(t, err, `refusing to permanently delete file "file_1" without --force`)
	require.ErrorContains(t, err, `use "everything-cli drive file trash <id>" instead`)
	require.False(t, svc.deleted)
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), "file_1", "--force")

	require.True(t, svc.deleted)
	require.Equal(t, "file_1", svc.deletedID)
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403: access denied")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "file_1", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
