package sheets

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

func TestDeleteWithoutForceRefuses(t *testing.T) {
	svc := &fakeFileService{}
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "sheet_1")

	require.Contains(t, err.Error(), "without --force")
	require.Contains(t, err.Error(), "cannot be undone")
	require.Empty(t, svc.deletedID)
}

func TestDeleteWithForceDeletesTheFile(t *testing.T) {
	svc := &fakeFileService{}
	cmdtest.RunCmd(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "sheet_1", "--force")

	require.True(t, svc.deleted)
	require.Equal(t, "sheet_1", svc.deletedID)
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeFileService{err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "sheet_1", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeFileService{}
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "a", "b")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
