package slides

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "pres_1")

	require.Contains(t, err.Error(), `refusing to permanently delete presentation "pres_1" without --force`)
	require.Contains(t, err.Error(), "cannot be undone")
	require.Contains(t, err.Error(), `use "everything-cli google drive file trash <id>" instead`)
	require.Empty(t, svc.DeletedIDs, "permanent delete must not reach the API without --force")
}

func TestDeleteWithForce(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	out := cmdtest.RunCmd(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "pres_1", "--force")

	require.Equal(t, []string{"pres_1"}, svc.DeletedIDs)
	require.Empty(t, out, "successful delete is silent")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &cmdtest.DeleteRecorder{Err: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "pres_1", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
