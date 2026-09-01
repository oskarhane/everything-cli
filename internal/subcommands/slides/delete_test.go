package slides

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeFileService{}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "pres_1")

	require.Contains(t, err.Error(), `refusing to permanently delete presentation "pres_1" without --force`)
	require.False(t, svc.deleted, "permanent delete must not reach the API without --force")
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeFileService{}
	out := cmdtest.RunCmd(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "pres_1", "--force")

	require.True(t, svc.deleted)
	require.Equal(t, "pres_1", svc.deletedID)
	require.Empty(t, out, "successful delete is silent")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeFileService{err: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "pres_1", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
