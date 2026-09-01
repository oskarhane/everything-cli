package sheets

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteWithoutForceRefuses(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "sheet_1")

	require.Contains(t, err.Error(), "without --force")
	require.Contains(t, err.Error(), "cannot be undone")
	require.Contains(t, err.Error(), `use "everything-cli drive file trash <id>" instead`)
	require.Empty(t, svc.DeletedIDs)
}

func TestDeleteWithForceDeletesTheFile(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	cmdtest.RunCmd(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "sheet_1", "--force")

	require.Equal(t, []string{"sheet_1"}, svc.DeletedIDs)
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &cmdtest.DeleteRecorder{Err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "sheet_1", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	_, err := cmdtest.RunCmdErr(t, newSheetCmd[service.FileService](newDeleteCmd, svc, "json"), "a", "b")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
